package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/appcli"
	"github.com/MikeO7/kinosail/packages/servertransport"
)

func TestTrustedDuckDNSCertificateIsSelectedOnlyForItsSNI(t *testing.T) { //nolint:cyclop // One TLS fixture proves trusted-SNI selection and local fallback.
	t.Parallel()
	dataDir := t.TempDir()
	hostname := "family-media.duckdns.org"
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(90 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeTestIdentity(filepath.Join(dataDir, "tls-duckdns.pem"), der, key); err != nil {
		t.Fatal(err)
	}
	raw := `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
	configured, err := configuration.Load(dataDir, "", func(name string) (string, bool) {
		values := map[string]string{"KINOSAIL_DATA_DIR": dataDir, "KINOSAIL_DUCKDNS_HTTPS": raw}
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := appcli.TrustedHTTPS(configured)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{ReadHeaderTimeout: time.Second}
	if err = servertransport.ConfigureCertificates(server, servertransport.TLSConfig{DataDir: dataDir, Certificates: manager}); err != nil {
		t.Fatal(err)
	}
	trustedCertificate, err := server.TLSConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: hostname})
	if err != nil || trustedCertificate.Leaf == nil || trustedCertificate.Leaf.VerifyHostname(hostname) != nil {
		t.Fatalf("trusted SNI certificate = %#v, %v", trustedCertificate, err)
	}
	fallback, err := server.TLSConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: "server.nox"})
	if err != nil || fallback == trustedCertificate {
		t.Fatalf("fallback certificate = %#v, %v", fallback, err)
	}
	fallbackLeaf, err := x509.ParseCertificate(fallback.Certificate[0])
	if err != nil || fallbackLeaf.Subject.CommonName != "Kinosail" {
		t.Fatalf("fallback leaf = %#v, %v", fallbackLeaf, err)
	}
}

func writeTestIdentity(path string, certificate []byte, key *ecdsa.PrivateKey) error {
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	contents := new(bytes.Buffer)
	_ = pem.Encode(contents, &pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	_ = pem.Encode(contents, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})
	return os.WriteFile(path, contents.Bytes(), 0o600)
}

func TestTrustedDuckDNSBecomesCanonicalAuthOrigin(t *testing.T) {
	t.Parallel()
	raw := `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
	for listen, want := range map[string]string{"127.0.0.1:38128": "https://family-media.duckdns.org:38128", ":443": "https://family-media.duckdns.org"} {
		configured, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
			values := map[string]string{"KINOSAIL_DUCKDNS_HTTPS": raw, "KINOSAIL_LISTEN": listen}
			value, ok := values[name]
			return value, ok
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := configuredAuthURL(configured); got != want {
			t.Fatalf("configuredAuthURL(%q) = %q, want %q", listen, got, want)
		}
	}
}

func TestTLSCertificateCommandExportsOnlyPublicCertificate(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	var output bytes.Buffer
	handled, err := command([]string{"tls-certificate"}, nil, &output, dataDir, "", "", false, "")
	if !handled || err != nil || !strings.Contains(output.String(), "BEGIN CERTIFICATE") || strings.Contains(output.String(), "PRIVATE KEY") {
		t.Fatalf("tls-certificate = %v, %v, %q", handled, err, output.String())
	}
	block, _ := pem.Decode(output.Bytes())
	certificate, parseErr := x509.ParseCertificate(block.Bytes)
	if parseErr != nil || certificate.Subject.CommonName != "Kinosail Local CA" || !certificate.IsCA {
		t.Fatalf("certificate = %#v, %v", certificate, parseErr)
	}
	var repeated bytes.Buffer
	if _, err = command([]string{"tls-certificate"}, nil, &repeated, dataDir, "", "", false, ""); err != nil || !bytes.Equal(output.Bytes(), repeated.Bytes()) {
		t.Fatalf("certificate was not persistent: %v", err)
	}
}

func TestCommandLineServesOwnerSuppliedRSACertificateToCompatibilityClient(t *testing.T) {
	address := unusedAddress(t)
	dataDir := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	contents := new(bytes.Buffer)
	_ = pem.Encode(contents, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = pem.Encode(contents, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})
	if err = os.WriteFile(filepath.Join(dataDir, "tls.pem"), contents.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+dataDir, "KINOSAIL_LISTEN="+address)
	command := startTestServer(t, environment)
	defer stopTestServer(command)
	pool := x509.NewCertPool()
	certificate, _ := x509.ParseCertificate(der)
	pool.AddCert(certificate)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}}}} //nolint:gosec // Exact-version client verifies the compatibility boundary.
	response, err := waitForResponse(t, client, "https://"+address+"/healthz")
	if err != nil {
		t.Fatalf("compatibility connection failed: %v", err)
	}
	_ = response.Body.Close()
	if response.TLS.Version != tls.VersionTLS12 || response.TLS.CipherSuite != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
		t.Fatalf("TLS = version %x suite %x", response.TLS.Version, response.TLS.CipherSuite)
	}
	if _, err = os.Stat(filepath.Join(dataDir, "tls-ca.pem")); !os.IsNotExist(err) {
		t.Fatalf("custom certificate created local authority: %v", err)
	}
}
