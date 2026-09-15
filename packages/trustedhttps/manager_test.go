package trustedhttps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestManagerObtainsAndServesTrustedCertificate(t *testing.T) { //nolint:cyclop // One test proves the complete obtain, persist, and serve lifecycle.
	t.Parallel()

	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	contents := testIdentity(t, config.Hostname(), now.Add(-time.Hour), now.Add(90*24*time.Hour))
	events := make([]string, 0, 2)
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		events = append(events, "address")
		assertDuckDNSQuery(t, request.URL.Query(), config, config.Address, "", false)
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duckDNS.Close()
	manager, err := New(config, t.TempDir(), Dependencies{
		Client:    duckDNS.Client(),
		UpdateURL: duckDNS.URL,
		Now:       func() time.Time { return now },
		obtain: func(_ context.Context, got Config, _ string, _ *http.Client, _ string, _ dnsProvider) ([]byte, error) {
			events = append(events, "obtain")
			if got != config {
				t.Fatalf("obtain arguments = %#v", got)
			}
			return contents, nil
		},
		Wait: func(context.Context, time.Duration) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.Run(context.Background())
	if !reflect.DeepEqual(events, []string{"address", "obtain"}) {
		t.Fatalf("events = %v", events)
	}
	status := manager.Status()
	if status.State != "ready" || status.Provider != ProviderDuckDNS || status.Hostname != config.Hostname() || status.Address != config.Address || status.CertificateExpires != now.Add(90*24*time.Hour).Format(time.RFC3339) || status.Error != "" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if strings.Contains(strings.Join([]string{status.State, status.Hostname, status.Address, status.CertificateExpires, status.Error}, " "), config.Token) {
		t.Fatal("status exposed DuckDNS token")
	}
	if manager.Certificate(config.Hostname()) == nil || manager.Certificate(strings.ToUpper(config.Hostname())+".") == nil {
		t.Fatal("trusted certificate was not served for its hostname")
	}
	if manager.Certificate("server.nox") != nil || manager.Certificate("") != nil {
		t.Fatal("trusted certificate was served for an unrelated hostname")
	}
	info, err := os.Stat(filepath.Join(manager.dataDir, certificateName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("certificate mode = %o", info.Mode().Perm())
	}
}

func TestManagerReusesValidCachedCertificate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	directory := t.TempDir()
	if err := writePrivate(filepath.Join(directory, certificateName), testIdentity(t, config.Hostname(), now.Add(-time.Hour), now.Add(90*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	updates := 0
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		updates++
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duckDNS.Close()
	manager, err := New(config, directory, Dependencies{
		Client:    duckDNS.Client(),
		UpdateURL: duckDNS.URL,
		Now:       func() time.Time { return now },
		obtain: func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
			t.Fatal("valid cached certificate was renewed")
			return nil, nil
		},
		Wait: func(context.Context, time.Duration) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.Status().State != "ready" {
		t.Fatalf("initial status = %#v", manager.Status())
	}
	manager.Run(context.Background())
	if updates != 1 {
		t.Fatalf("DuckDNS updates = %d, want 1", updates)
	}
}

func TestManagerReplacesInvalidCachedCertificate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	config := Config{Domain: "family", Token: testToken, Address: "192.168.1.10", Terms: true}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, certificateName), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	duckDNS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("OK")) }))
	defer duckDNS.Close()
	obtained := false
	manager, err := New(config, directory, Dependencies{
		Client:    duckDNS.Client(),
		UpdateURL: duckDNS.URL,
		Now:       func() time.Time { return now },
		obtain: func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
			obtained = true
			return testIdentity(t, config.Hostname(), now.Add(-time.Hour), now.Add(90*24*time.Hour)), nil
		},
		Wait: func(context.Context, time.Duration) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.Status().State != "starting" || manager.Certificate(config.Hostname()) != nil {
		t.Fatalf("invalid cache should not be active: %#v", manager.Status())
	}
	manager.Run(context.Background())
	if !obtained || manager.Status().State != "ready" {
		t.Fatalf("invalid cache was not replaced: %#v", manager.Status())
	}
}

func TestNewRejectsInvalidConfigWithoutSideEffects(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "must-not-exist")
	called := false
	_, err := New(Config{Domain: "family", Token: "short", Address: "192.168.1.10", Terms: true}, directory, Dependencies{
		obtain: func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
			called = true
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("New accepted invalid configuration")
	}
	if called {
		t.Fatal("invalid configuration reached certificate adapter")
	}
	if _, statErr := os.Stat(directory); !os.IsNotExist(statErr) {
		t.Fatalf("invalid configuration changed filesystem: %v", statErr)
	}
}

func testIdentity(t *testing.T, hostname string, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: hostname},
		DNSNames:     []string{hostname},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})...)
}

func assertDuckDNSQuery(t *testing.T, query url.Values, config Config, address, txt string, clear bool) {
	t.Helper()
	if query.Get("domains") != config.Domain || query.Get("token") != config.Token || query.Get("ip") != address || query.Get("txt") != txt {
		t.Fatalf("unexpected DuckDNS query: %v", query)
	}
	if (query.Get("clear") == "true") != clear {
		t.Fatalf("clear = %q, want %v", query.Get("clear"), clear)
	}
}
