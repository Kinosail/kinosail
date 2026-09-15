package publicgateway

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestGatewayForwardsOnlyToPublicSocketAndStripsProxyCredentials(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "gateway-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "http.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	backend := &http.Server{Handler: TLSMetadata(identitycore.Remote(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !identitycore.RemoteRequest(r) || r.TLS == nil || r.Header.Get("X-Kinosail-Proxy-Token") != "" || r.Header.Get("X-Forwarded-For") != "" || r.RemoteAddr != "198.51.100.10:5000" || r.Header.Get("Authorization") != "Bearer viewer" {
			t.Error("gateway boundary lost")
		}
		w.WriteHeader(http.StatusNoContent)
	})))}
	go func() { _ = backend.Serve(listener) }()
	t.Cleanup(func() { _ = backend.Close() })
	r := httptest.NewRequest("GET", "https://family.duckdns.org/api/v1/items", nil)
	r.URL.Scheme = ""
	r.URL.Host = ""
	r.RemoteAddr = "198.51.100.10:5000"
	r.Header.Set("X-Kinosail-Proxy-Token", "forged")
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	r.Header.Set(ClientAddressHeader, "127.0.0.1:1")
	r.Header.Set("Authorization", "Bearer viewer")
	w := httptest.NewRecorder()
	Handler("family.duckdns.org", path).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("proxy = %d %s", w.Code, w.Body.String())
	}
}

func TestGatewayRejectsWrongHostPlaintextOversizeAndTransferEncoding(t *testing.T) {
	for name, mutate := range map[string]func(*http.Request){"host": func(r *http.Request) { r.Host = "other.example" }, "plaintext": func(r *http.Request) { r.TLS = nil }, "large": func(r *http.Request) { r.ContentLength = 1<<20 + 1 }, "chunked": func(r *http.Request) { r.TransferEncoding = []string{"chunked"} }, "connect": func(r *http.Request) { r.Method = http.MethodConnect }, "absolute": func(r *http.Request) { r.URL.Host = "other.example"; r.URL.Scheme = "http" }} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://family.duckdns.org/", nil)
			r.URL.Scheme = ""
			r.URL.Host = ""
			mutate(r)
			w := httptest.NewRecorder()
			Handler("family.duckdns.org", "/must-not-connect.sock").ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest && w.Code != http.StatusMisdirectedRequest {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestCertificateRPCRejectsInvalidInputBeforeIssuance(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"name":"other.example"}`, `{"name":"family.duckdns.org","name":"family.duckdns.org"}`, `{"name":"family.duckdns.org","acme":"true"}`, `{"name":"family.duckdns.org","key":"owner"}`, strings.Repeat(" ", 1025)} {
		called := false
		handler := CertificateHandler("family.duckdns.org", func(*tls.ClientHelloInfo) (*tls.Certificate, error) { called = true; return nil, nil })
		r := httptest.NewRequest("POST", "http://certificate/", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if called || w.Code != http.StatusBadRequest {
			t.Fatalf("accepted %q: %d", raw, w.Code)
		}
	}
}

func TestGatewayMetadataRejectsMalformedPeerBeforeApplication(t *testing.T) {
	for _, address := range []string{"", "127.0.0.1", "host.example:10", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:010", "127.0.0.1:bad", strings.Repeat("x", 257)} {
		called := false
		handler := TLSMetadata(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set(ClientAddressHeader, address)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if called || w.Code != http.StatusBadRequest {
			t.Fatalf("invalid gateway peer %q was accepted", address)
		}
	}
}
