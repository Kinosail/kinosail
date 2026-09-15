package remoteaccess_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"golang.org/x/net/http2"
)

func TestPublicHTTP1RejectsAmbiguousFramingBeforeHandlerSideEffects(t *testing.T) {
	t.Parallel()
	var handled atomic.Int32
	address, cancel := startProtocolServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := io.ReadAll(request.Body); err != nil {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		handled.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer cancel()
	for name, raw := range map[string]string{
		"conflicting content length":   "POST / HTTP/1.1\r\nHost: family-media.duckdns.org\r\nContent-Length: 1\r\nContent-Length: 2\r\n\r\nx",
		"transfer encoding and length": "POST / HTTP/1.1\r\nHost: family-media.duckdns.org\r\nTransfer-Encoding: chunked\r\nContent-Length: 4\r\n\r\n0\r\n\r\n",
		"folded framing header":        "POST / HTTP/1.1\r\nHost: family-media.duckdns.org\r\nContent-Length: 0\r\n Transfer-Encoding: chunked\r\n\r\n",
		"invalid chunk":                "POST / HTTP/1.1\r\nHost: family-media.duckdns.org\r\nTransfer-Encoding: chunked\r\n\r\nZZ\r\nbody\r\n0\r\n\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			connection := protocolTLSConnection(t, address)
			defer connection.Close()
			_ = connection.SetDeadline(time.Now().Add(time.Second))
			if _, err := io.WriteString(connection, raw); err != nil {
				t.Fatal(err)
			}
			response, _ := io.ReadAll(connection)
			if !bytes.Contains(response, []byte(" 400 ")) || handled.Load() != 0 {
				t.Fatalf("response=%q handled=%d", response, handled.Load())
			}
		})
	}
}

func TestPublicHTTP2AdvertisesExplicitResourceBudgets(t *testing.T) { //nolint:cyclop,gocognit // One wire-level exchange verifies every advertised HTTP/2 resource budget.
	t.Parallel()
	address, cancel := startProtocolServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	defer cancel()
	connection, err := (&tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13, NextProtos: []string{"h2"}}}).DialContext(t.Context(), "tcp", address) //nolint:gosec // Private fixture certificate.
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err = io.WriteString(connection, http2.ClientPreface); err != nil {
		t.Fatal(err)
	}
	framer := http2.NewFramer(connection, connection)
	if err = framer.WriteSettings(); err != nil {
		t.Fatal(err)
	}
	want := map[http2.SettingID]uint32{http2.SettingMaxConcurrentStreams: 32, http2.SettingHeaderTableSize: 4096, http2.SettingMaxFrameSize: 16 << 10, http2.SettingInitialWindowSize: 256 << 10}
	for {
		frame, readErr := framer.ReadFrame()
		if readErr != nil {
			t.Fatal(readErr)
		}
		settings, ok := frame.(*http2.SettingsFrame)
		if !ok || settings.IsAck() {
			continue
		}
		seen := make(map[http2.SettingID]uint32)
		if err = settings.ForeachSetting(func(setting http2.Setting) error { seen[setting.ID] = setting.Val; return nil }); err != nil {
			t.Fatal(err)
		}
		for id, expected := range want {
			if seen[id] != expected {
				t.Fatalf("HTTP/2 setting %v = %d, want %d", id, seen[id], expected)
			}
		}
		return
	}
}

func TestPublicTLSPreservesHybridAndClassicalInteroperability(t *testing.T) {
	t.Parallel()
	address, cancel := startProtocolServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	defer cancel()
	for name, curves := range map[string][]tls.CurveID{"hybrid ML-KEM": {tls.X25519MLKEM768}, "classical X25519": {tls.X25519}} {
		t.Run(name, func(t *testing.T) {
			dialer := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13, CurvePreferences: curves}} //nolint:gosec // Private fixture certificate.
			connection, err := dialer.DialContext(t.Context(), "tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			_ = connection.Close()
		})
	}
}

func startProtocolServer(t *testing.T, handler http.Handler) (string, context.CancelFunc) {
	t.Helper()
	address := unusedAddress(t)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("OK")), Header: make(http.Header)}, nil
	})}
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("a", 32), Listen: address, DataDir: t.TempDir()}, remoteaccess.Dependencies{Client: client, Certificate: testCertificate(t)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	go func() { _ = manager.Serve(ctx, handler) }()
	for range 100 {
		connection, dialErr := (&tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13}}).DialContext(t.Context(), "tcp", address) //nolint:gosec // Private fixture certificate.
		if dialErr == nil {
			_ = connection.Close()
			return address, cancel
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("public protocol fixture did not start")
	return "", func() {}
}

func protocolTLSConnection(t *testing.T, address string) net.Conn {
	t.Helper()
	connection, err := (&tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13, NextProtos: []string{"http/1.1"}}}).DialContext(t.Context(), "tcp", address) //nolint:gosec // Private fixture certificate.
	if err != nil {
		t.Fatal(err)
	}
	return connection
}
