package servertransport

import (
	"bytes"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewServerAppliesSharedTransportLimits(t *testing.T) { //nolint:cyclop // The score of 19 remains below the repository ceiling of 22 for the transport matrix.
	server := NewServer("127.0.0.1:38127", http.NotFoundHandler())
	if server.Addr != "127.0.0.1:38127" || server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.IdleTimeout != 2*time.Minute || server.WriteTimeout != 0 {
		t.Fatalf("HTTP limits = %#v", server)
	}
	if server.MaxHeaderBytes != 64<<10 || server.MaxHeaderValueCount != 64 || server.ErrorLog == nil || server.HTTP2 == nil {
		t.Fatalf("HTTP header limits = %#v", server)
	}
	http2 := server.HTTP2
	if http2.MaxConcurrentStreams != 32 || http2.MaxDecoderHeaderTableSize != 4096 || http2.MaxEncoderHeaderTableSize != 4096 || http2.MaxReadFrameSize != 16<<10 || http2.MaxReceiveBufferPerConnection != 1<<20 || http2.MaxReceiveBufferPerStream != 256<<10 {
		t.Fatalf("HTTP/2 capacity limits = %#v", http2)
	}
	if http2.SendPingTimeout != 30*time.Second || http2.PingTimeout != 10*time.Second || http2.WriteByteTimeout != 30*time.Second {
		t.Fatalf("HTTP/2 timeouts = %#v", http2)
	}
}

func TestServerErrorLogRedactsClientAddress(t *testing.T) {
	server := NewServer("127.0.0.1:38127", http.NotFoundHandler())
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	server.ErrorLog.Print("http: TLS handshake error from 192.168.1.86:63174: remote error: tls: unknown certificate")
	server.ErrorLog.Print("http: accept error serving [2001:db8::1]:443: closed")
	logged := output.String()
	if !strings.Contains(logged, `"msg":"HTTP transport error"`) || !strings.Contains(logged, `"detail":"http: TLS handshake error from [redacted]: remote error: tls: unknown certificate"`) || strings.Contains(logged, "192.168.1.86") || strings.Contains(logged, "63174") || strings.Contains(logged, "2001:db8") || strings.Contains(logged, ":443") {
		t.Fatalf("transport log exposed client address: %s", logged)
	}
}

func TestServeSelectsProtocolAndReportsCertificateFailure(t *testing.T) {
	t.Parallel()
	errPlain, errSecure := errors.New("plain stopped"), errors.New("secure stopped")
	server := NewServer("", http.NotFoundHandler())
	if err := serve(server, TLSConfig{}, func() error { return errPlain }, func() error { return nil }); !errors.Is(err, errPlain) {
		t.Fatalf("plain serve error = %v", err)
	}
	if err := serve(server, TLSConfig{Enabled: true, DataDir: ""}, func() error { return nil }, func() error { return errSecure }); err == nil {
		t.Fatal("invalid TLS configuration was served")
	}
	if err := serve(server, TLSConfig{Enabled: true, DataDir: t.TempDir()}, func() error { return nil }, func() error { return errSecure }); !errors.Is(err, errSecure) {
		t.Fatalf("secure serve error = %v", err)
	}
}

func TestServeRunsPlainAndSecureListeners(t *testing.T) {
	for _, config := range []TLSConfig{{}, {Enabled: true, DataDir: t.TempDir()}} {
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		_ = listener.Close()
		server := NewServer(address, http.NotFoundHandler())
		result := make(chan error, 1)
		go func() { result <- Serve(server, config) }()
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			connection, dialErr := (&net.Dialer{Timeout: 10 * time.Millisecond}).DialContext(t.Context(), "tcp", address)
			if dialErr == nil {
				_ = connection.Close()
				break
			}
		}
		if err := server.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve() error = %v", err)
		}
	}
}

func TestConfigureCertificatesInitializesMissingTLSConfig(t *testing.T) {
	t.Parallel()
	server := &http.Server{ReadHeaderTimeout: time.Second}
	if err := ConfigureCertificates(server, TLSConfig{DataDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if server.TLSConfig == nil || len(server.TLSConfig.Certificates) != 1 {
		t.Fatalf("TLS config = %#v", server.TLSConfig)
	}
}

func TestCheckHealthCoversProtocolsAndInputFailures(t *testing.T) { //nolint:cyclop // Protocol and error branches share one loopback boundary.
	hostSeen := ""
	plain := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hostSeen = request.Host
		writer.WriteHeader(http.StatusOK)
	}))
	defer plain.Close()
	if err := CheckHealth(plain.Listener.Addr().String(), false, "https://family.example:38127"); err != nil || hostSeen != "family.example:38127" {
		t.Fatalf("plain health = %q, %v", hostSeen, err)
	}
	secure := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	secure.TLS = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}
	secure.StartTLS()
	defer secure.Close()
	if err := CheckHealth(secure.Listener.Addr().String(), true, ""); err != nil {
		t.Fatal(err)
	}
	badStatus := httptest.NewServer(http.NotFoundHandler())
	defer badStatus.Close()
	for _, test := range []struct {
		listen, auth string
	}{
		{badStatus.Listener.Addr().String(), ""},
		{"invalid", ""},
		{plain.Listener.Addr().String(), "https:///"},
	} {
		if err := CheckHealth(test.listen, false, test.auth); err == nil {
			t.Fatalf("invalid health input was accepted: %#v", test)
		}
	}
	if err := CheckHealth("127.0.0.1:%", false, ""); err == nil {
		t.Fatal("invalid health URL was accepted")
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closedAddress := closed.Listener.Addr().String()
	closed.Close()
	if err := CheckHealth(closedAddress, false, ""); err == nil {
		t.Fatal("unreachable health endpoint was accepted")
	}
}
