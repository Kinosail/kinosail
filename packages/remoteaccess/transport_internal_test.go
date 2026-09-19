package remoteaccess

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

func TestServeHandlesInactiveAndKilledManagers(t *testing.T) {
	disabled, err := New(Config{})
	if err != nil || disabled.Serve(t.Context(), http.NotFoundHandler()) != nil {
		t.Fatalf("disabled serve = %#v, %v", disabled, err)
	}
	manager := activeManager(t)
	manager.killed = true
	if err = manager.Serve(t.Context(), http.NotFoundHandler()); err != nil {
		t.Fatalf("killed serve = %v", err)
	}
}

func TestServeRetriesUpdatesAndStopsWithContext(t *testing.T) {
	manager := activeManager(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	calls := 0
	manager.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("offline")
		}
		cancel()
		return okResponse(), nil
	})}
	retry := make(chan time.Time, 1)
	retry <- time.Time{}
	manager.operations.after = func(time.Duration) <-chan time.Time { return retry }
	if err := manager.Serve(ctx, http.NotFoundHandler()); err != nil || calls != 2 {
		t.Fatalf("retry serve = %v, calls=%d", err, calls)
	}

	manager = activeManager(t)
	manager.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	canceled, stop := context.WithCancel(t.Context())
	stop()
	if err := manager.Serve(canceled, http.NotFoundHandler()); err != nil {
		t.Fatalf("canceled update = %v", err)
	}
}

func TestServeReportsConfigurationListenAndAcceptFailures(t *testing.T) {
	want := errors.New("configure failed")
	manager := activeManager(t)
	manager.operations.configureServer = func(*http.Server, *http2.Server) error { return want }
	if err := manager.Serve(t.Context(), http.NotFoundHandler()); !errors.Is(err, want) || manager.Status().State != "error" {
		t.Fatalf("configure failure = %v, %#v", err, manager.Status())
	}

	want = errors.New("listen failed")
	manager = activeManager(t)
	manager.operations.listen = func(context.Context, string, string) (net.Listener, error) { return nil, want }
	if err := manager.Serve(t.Context(), http.NotFoundHandler()); !errors.Is(err, want) || manager.Status().State != "error" {
		t.Fatalf("listen failure = %v, %#v", err, manager.Status())
	}

	listener := &failureListener{acceptErr: errors.New("accept failed")}
	manager = activeManager(t)
	manager.operations.listen = func(context.Context, string, string) (net.Listener, error) { return listener, nil }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := manager.Serve(ctx, http.NotFoundHandler()); !errors.Is(err, listener.acceptErr) || manager.Status().State != "error" {
		t.Fatalf("accept failure = %v, %#v", err, manager.Status())
	}
	if manager.server != nil || manager.listener != nil {
		t.Fatal("failed server references were retained")
	}
}

func TestServeClosesListenerIfKilledDuringStartup(t *testing.T) {
	manager := activeManager(t)
	listener := &failureListener{}
	manager.operations.listen = func(context.Context, string, string) (net.Listener, error) {
		manager.mu.Lock()
		manager.killed = true
		manager.mu.Unlock()
		return listener, nil
	}
	if err := manager.Serve(t.Context(), http.NotFoundHandler()); err != nil || !listener.closed {
		t.Fatalf("killed startup = %v, closed=%t", err, listener.closed)
	}
}

func TestRefreshRecordsFailureAndSuccess(t *testing.T) {
	for name, result := range map[string]error{"failure": errors.New("offline"), "success": nil} {
		t.Run(name, func(t *testing.T) {
			manager := activeManager(t)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			manager.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				cancel()
				if result != nil {
					return nil, result
				}
				return okResponse(), nil
			})}
			ticks := make(chan time.Time, 1)
			ticks <- time.Time{}
			stopped := false
			manager.operations.refreshTicks = func() (<-chan time.Time, func()) { return ticks, func() { stopped = true } }
			manager.refresh(ctx)
			want := "ready"
			if result != nil {
				want = "error"
			}
			if manager.Status().State != want || !stopped {
				t.Fatalf("refresh = %#v, stopped=%t", manager.Status(), stopped)
			}
		})
	}
}

func TestUpdateRejectsRequestNetworkAndResponseFailures(t *testing.T) {
	manager := activeManager(t)
	manager.updateURL = "%"
	if err := manager.update(t.Context()); err == nil {
		t.Fatal("invalid request URL accepted")
	}
	manager = activeManager(t)
	if err := manager.update(t.Context()); err != nil || manager.Status().LastDDNSUpdate == "" {
		t.Fatalf("valid update = %v, %#v", err, manager.Status())
	}
	for name, client := range map[string]*http.Client{
		"network": {Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })},
		"body": {Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: failingBody{}}, nil
		})},
		"status": {Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			response := okResponse()
			response.StatusCode = http.StatusForbidden
			return response, nil
		})},
		"oversized": {Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("O", 65)))}, nil
		})},
		"content": {Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("NO"))}, nil
		})},
	} {
		t.Run(name, func(t *testing.T) {
			manager := activeManager(t)
			manager.client = client
			if err := manager.update(t.Context()); err == nil {
				t.Fatal("failed DuckDNS response accepted")
			}
		})
	}
}

func TestTrackConnectionEnforcesSourceAndGlobalBudgets(t *testing.T) {
	manager := activeManager(t)
	connection := &stubConn{remote: stubAddress("192.0.2.1:4000")}
	manager.trackConnection(connection, http.StateNew)
	manager.trackConnection(connection, http.StateActive)
	if manager.Status().Connections != 1 {
		t.Fatalf("active connections = %d", manager.Status().Connections)
	}
	manager.trackConnection(connection, http.StateHijacked)
	if manager.Status().Connections != 0 {
		t.Fatalf("connections after hijack = %d", manager.Status().Connections)
	}

	manager.sources["192.0.2.1"] = 32
	rejected := &stubConn{remote: stubAddress("192.0.2.1:4001")}
	manager.trackConnection(rejected, http.StateNew)
	if !rejected.closed {
		t.Fatal("source connection budget was not enforced")
	}

	manager = activeManager(t)
	for index := range 256 {
		existing := &stubConn{remote: stubAddress("198.51.100.1:4000")}
		manager.connections[existing] = string(rune(index + 1))
	}
	rejected = &stubConn{remote: stubAddress("198.51.100.2:4000")}
	manager.trackConnection(rejected, http.StateNew)
	if !rejected.closed {
		t.Fatal("global connection budget was not enforced")
	}

	manager = activeManager(t)
	manager.killed = true
	rejected = &stubConn{remote: stubAddress("not-a-host-port")}
	manager.trackConnection(rejected, http.StateNew)
	manager.trackConnection(rejected, http.StateClosed)
	if !rejected.closed || remoteHost("not-a-host-port") != "not-a-host-port" || remoteHost("[2001:db8::1]:443") != "2001:db8::1" {
		t.Fatal("killed or remote host handling failed")
	}
}

func activeManager(t *testing.T) *Manager {
	t.Helper()
	config := Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: t.TempDir()}
	manager, err := New(config, Dependencies{Client: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return okResponse(), nil })}, Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return validLeaf("family.duckdns.org", time.Now().Add(-time.Minute), time.Now().Add(time.Hour)), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

type transportFunc func(*http.Request) (*http.Response, error)

func (function transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func okResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("OK")), Header: make(http.Header)}
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingBody) Close() error             { return nil }

type failureListener struct {
	acceptErr error
	closed    bool
}

func (listener *failureListener) Accept() (net.Conn, error) { return nil, listener.acceptErr }
func (listener *failureListener) Close() error              { listener.closed = true; return nil }
func (*failureListener) Addr() net.Addr                     { return stubAddress("127.0.0.1:8443") }

type stubAddress string

func (address stubAddress) Network() string { return "tcp" }
func (address stubAddress) String() string  { return string(address) }

type stubConn struct {
	remote net.Addr
	closed bool
}

func (*stubConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (*stubConn) Write(data []byte) (int, error)   { return len(data), nil }
func (connection *stubConn) Close() error          { connection.closed = true; return nil }
func (*stubConn) LocalAddr() net.Addr              { return stubAddress("127.0.0.1:8443") }
func (connection *stubConn) RemoteAddr() net.Addr  { return connection.remote }
func (*stubConn) SetDeadline(time.Time) error      { return nil }
func (*stubConn) SetReadDeadline(time.Time) error  { return nil }
func (*stubConn) SetWriteDeadline(time.Time) error { return nil }
