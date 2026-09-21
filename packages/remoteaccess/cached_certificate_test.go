package remoteaccess

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func TestCachedCertificateNeverIssuesAndFallsBackToRSAIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	hostname := server.Certificate().DNSNames[0]
	manager := &Manager{config: Config{PublicHTTPS: true, DataDir: t.TempDir()}, hostname: hostname, certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		t.Fatal("cache lookup invoked issuer")
		return nil, nil
	}}
	for _, candidate := range []*Manager{nil, {}, manager} {
		if candidate.Certificate("wrong.example") != nil {
			t.Fatal("unexpected identity returned")
		}
	}
	if manager.Certificate(hostname) != nil {
		t.Fatal("missing identity returned")
	}
	if err := privatefile.Write(filepath.Join(manager.config.DataDir, "acme", hostname), []byte("invalid PEM")); err != nil {
		t.Fatal(err)
	}
	material := server.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(material.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	content := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: material.Certificate[0]}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})...)
	if err := privatefile.Write(filepath.Join(manager.config.DataDir, "acme", hostname+"+rsa"), content); err != nil {
		t.Fatal(err)
	}
	cached := manager.Certificate(hostname)
	if cached == nil || cached.Leaf.VerifyHostname(hostname) != nil {
		t.Fatal("valid cached identity unavailable")
	}
}
