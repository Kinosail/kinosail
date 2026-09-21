package publicgateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gatewayCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "family.example"}, DNSNames: []string{"family.example"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func certificateSocket(t *testing.T, handler http.Handler) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "certificate-rpc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "cert.sock")
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return path
}

func TestCertificateClientPerformsTrustedTLSHandshake(t *testing.T) {
	cert := gatewayCertificate(t)
	requested := make(chan *tls.ClientHelloInfo, 1)
	path := certificateSocket(t, CertificateHandler("family.example", func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) { requested <- hello; return &cert, nil }))
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: CertificateClient("family.example", path)}
	server.StartTLS()
	t.Cleanup(server.Close)
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "family.example"}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	request.RequestURI = ""
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d", response.StatusCode)
	}
	hello := <-requested
	if hello.ServerName != "family.example" || !ecdsaCapable(hello) {
		t.Fatal("certificate RPC lost negotiation")
	}
}

func TestCertificateClientRejectsNameAndMissingContext(t *testing.T) {
	client := CertificateClient("family.example", "/must-not-connect.sock")
	for _, name := range []string{"attacker.example", "family.example"} {
		if cert, err := client(&tls.ClientHelloInfo{ServerName: name}); err == nil || cert != nil {
			t.Fatal("invalid hello accepted")
		}
	}
}

func TestCertificateClientRejectsInvalidRPCResponses(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"unavailable", http.StatusServiceUnavailable, "application/x-pem-file", "private failure"},
		{"wrong type", http.StatusOK, "text/plain", "not a certificate"},
		{"invalid PEM", http.StatusOK, "application/x-pem-file", "invalid"},
		{"oversized", http.StatusOK, "application/x-pem-file", strings.Repeat("x", 256<<10+1)},
		{"redirect", http.StatusFound, "application/x-pem-file", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := make(chan bool, 1)
			path := certificateSocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requested <- true
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Location", "http://must-not-follow.invalid")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			server := httptest.NewUnstartedServer(http.NotFoundHandler())
			server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: CertificateClient("family.example", path)}
			server.StartTLS()
			defer server.Close()
			_, address, _ := strings.Cut(server.URL, "https://")
			dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "family.example"}}
			connection, err := dialer.DialContext(t.Context(), "tcp", address)
			if connection != nil {
				_ = connection.Close()
			}
			if err == nil {
				t.Fatal("invalid certificate response accepted")
			}
			select {
			case <-requested:
			default:
				t.Fatal("handshake did not exercise certificate RPC")
			}
		})
	}
}
