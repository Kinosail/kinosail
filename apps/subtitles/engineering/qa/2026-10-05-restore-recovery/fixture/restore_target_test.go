package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// The socket target is allocated before any root input is read. Destination
// URLs come from this finite inventory, never from a fixture or request URL.
type restoreTarget struct {
	tls               *httptest.Server
	listener          *restoreCheckedListener
	origin, authority string
	mu                sync.Mutex
	routes            map[string]url.URL
	item, receipt     string
	stopOnce          sync.Once
	closeOK           bool
	controlClient     *http.Client
	controlCSRF       string
	causalProbe       [3]restoreProbeSnapshot
}

func newOwnedRestoreTarget() (*restoreTarget, error) {
	tls := httptest.NewUnstartedServer(nil)
	listener := &restoreCheckedListener{Listener: tls.Listener}
	tls.Listener = listener
	authority := listener.Addr().String()
	target := &restoreTarget{
		tls: tls, listener: listener, authority: authority,
		origin: "https://" + authority, routes: make(map[string]url.URL),
		closeOK: true,
	}
	if !ownedLoopbackAuthority(authority) {
		if !target.stop() {
			return nil, errors.New("fixture listener close failed")
		}
		return nil, errors.New("fixture listener is not owned loopback")
	}
	target.seedFixedRoutes()
	return target, nil
}

func ownedLoopbackAuthority(authority string) bool {
	host, port, err := net.SplitHostPort(authority)
	if err != nil || port == "" {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (target *restoreTarget) endpoint(path string) (string, error) {
	target.mu.Lock()
	endpoint, exists := target.routes[path]
	target.mu.Unlock()
	if !exists {
		return "", errors.New("private route is not admitted")
	}
	return endpoint.String(), nil
}

func (target *restoreTarget) ownsPath(path string) bool {
	target.mu.Lock()
	_, exists := target.routes[path]
	target.mu.Unlock()
	return exists
}

func (target *restoreTarget) ownsItem(id string) bool {
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.item != "" && id == target.item
}

func (target *restoreTarget) privateClient(timeout time.Duration) *http.Client {
	client := *target.tls.Client()
	original, ok := client.Transport.(*http.Transport)
	if !ok {
		client.Transport = deniedTransport{}
		return &client
	}
	transport := original.Clone()
	transport.DialContext = target.dialOwned
	client.Transport = transport
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func (target *restoreTarget) dialOwned(ctx context.Context, network, address string) (net.Conn, error) {
	if address != target.authority {
		return nil, errors.New("private transport authority rejected")
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	return dialer.DialContext(ctx, network, address)
}

func (target *restoreTarget) admittedRequest(request *http.Request) bool {
	if request.URL == nil {
		return false
	}
	endpoint := request.URL
	if endpoint.Scheme != "https" || endpoint.Host != target.authority || endpoint.User != nil {
		return false
	}
	address := net.ParseIP(endpoint.Hostname())
	if address == nil || !address.IsLoopback() {
		return false
	}
	return target.ownsPath(endpoint.RequestURI()) && endpoint.Fragment == "" && endpoint.Opaque == "" && endpoint.RawPath == ""
}

type deniedTransport struct{}

func (deniedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("private TLS transport unavailable")
}

func newRestoreTarget(t *testing.T) *restoreTarget {
	t.Helper()
	target, err := newOwnedRestoreTarget()
	if err != nil {
		t.Fatal("Restore independent loopback target unavailable")
	}
	t.Cleanup(func() {
		if !target.stop() {
			t.Error("Restore owned listener did not close")
		}
	})
	return target
}

type restoreCheckedListener struct {
	net.Listener
	closeOnce sync.Once
	closeErr  error
}

func (listener *restoreCheckedListener) Close() error {
	listener.closeOnce.Do(func() { listener.closeErr = listener.Listener.Close() })
	return listener.closeErr
}

func (target *restoreTarget) stop() bool {
	target.stopOnce.Do(func() {
		target.tls.CloseClientConnections()
		target.tls.Close()
		target.closeOK = target.listener.Close() == nil
	})
	return target.closeOK
}

func (target *restoreTarget) installControlOwner(client *http.Client, csrf string) {
	target.mu.Lock()
	defer target.mu.Unlock()
	target.controlClient, target.controlCSRF = client, csrf
}

func (target *restoreTarget) controlOwner() (*http.Client, string) {
	target.mu.Lock()
	defer target.mu.Unlock()
	return target.controlClient, target.controlCSRF
}
