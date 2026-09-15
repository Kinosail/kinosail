package remoteaccess_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestSecureInternetAccessUpdatesDuckDNSThenServesOnlyTLS(t *testing.T) { //nolint:cyclop,funlen,gocognit // Scores of 16 and 17 remain below the repository ceiling of 22 for the secure-access lifecycle.
	t.Parallel()
	updates := make(chan url.Values, 1)
	activeStarted, activeCanceled := make(chan struct{}), make(chan struct{})
	duck := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		updates <- request.URL.Query()
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duck.Close()
	address := unusedAddress(t)
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("a", 32), Listen: address, DataDir: t.TempDir()}, remoteaccess.Dependencies{Client: duck.Client(), UpdateURL: duck.URL, Certificate: testCertificate(t)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = manager.Serve(ctx, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/hold" {
				writer.(http.Flusher).Flush()
				close(activeStarted)
				<-request.Context().Done()
				close(activeCanceled)
				return
			}
			_, _ = writer.Write([]byte("accepted"))
		}))
	}()

	select {
	case query := <-updates:
		if query.Get("domains") != "family-media" || query.Get("token") != strings.Repeat("a", 32) || query.Get("ip") != "" {
			t.Fatalf("DuckDNS update = %#v", query)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DuckDNS was not updated")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13}}} //nolint:gosec // Test certificate is intentionally private.
	var response *http.Response
	for range 100 {
		request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+address, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Host = "family-media.duckdns.org"
		response, err = client.Do(request)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.TLS.Version != tls.VersionTLS13 || manager.Status().State != "ready" {
		t.Fatalf("HTTPS=%d TLS=%x status=%#v", response.StatusCode, response.TLS.Version, manager.Status())
	}
	if manager.Status().Mode != "https" {
		t.Fatalf("HTTPS mode status = %#v", manager.Status())
	}
	assertUntrustedPublicRequestsRejected(t, client, address)
	assertKillClosesPublicAccess(t, manager, client, address, activeStarted, activeCanceled)
}

func TestWireGuardModeUpdatesDuckDNSWithoutOpeningPublicHTTPS(t *testing.T) {
	t.Parallel()
	updated := make(chan struct{}, 1)
	duck := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		updated <- struct{}{}
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duck.Close()
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, Domain: "private-family", Token: strings.Repeat("b", 32)}, remoteaccess.Dependencies{Client: duck.Client(), UpdateURL: duck.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- manager.Serve(ctx, http.NotFoundHandler()) }()
	select {
	case <-updated:
	case <-time.After(2 * time.Second):
		t.Fatal("DuckDNS was not updated")
	}
	for range 100 {
		if manager.Status().State == "ready" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if manager.Status().State != "ready" || manager.Status().Mode != "wireguard" {
		t.Fatalf("status = %#v", manager.Status())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDisabledInternetAccessReportsOffMode(t *testing.T) {
	t.Parallel()
	manager, err := remoteaccess.New(remoteaccess.Config{})
	if err != nil || manager.Status().State != "disabled" || manager.Status().Mode != "off" {
		t.Fatalf("disabled status = %#v, %v", manager.Status(), err)
	}
}

func TestKillClosesInMemoryAccessEvenWhenPersistenceFails(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("a", 32), Listen: unusedAddress(t), DataDir: directory}, remoteaccess.Dependencies{Certificate: testCertificate(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Kill(); err == nil || manager.Status().State != "killed" {
		t.Fatalf("failed persistent kill status = %#v, %v", manager.Status(), err)
	}
}

func TestPersistedKillStartsLocalAppWithoutInitializingCertificates(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "remote-access.disabled"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "acme"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: directory})
	if err != nil || manager.Status().State != "killed" {
		t.Fatalf("persisted kill = %#v, %v", manager, err)
	}
}

func TestSecureInternetAccessRejectsInvalidConfigurationWithoutNetworkActivity(t *testing.T) {
	t.Parallel()
	called := false
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { called = true; return nil, nil })}
	for name, config := range map[string]remoteaccess.Config{
		"missing domain": {Enabled: true, PublicHTTPS: true, Token: strings.Repeat("a", 32), Listen: ":8443", DataDir: t.TempDir()},
		"full hostname":  {Enabled: true, PublicHTTPS: true, Domain: "bad.duckdns.org", Token: strings.Repeat("a", 32), Listen: ":8443", DataDir: t.TempDir()},
		"short token":    {Enabled: true, PublicHTTPS: true, Domain: "family", Token: "short", Listen: ":8443", DataDir: t.TempDir()},
		"bad listen":     {Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "public", DataDir: t.TempDir()},
		"missing data":   {Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: ":8443"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := remoteaccess.New(config, remoteaccess.Dependencies{Client: client}); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
	if called {
		t.Fatal("invalid configuration caused network activity")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func doGet(t *testing.T, client *http.Client, target string) (*http.Response, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return client.Do(request)
}

func testCertificate(t *testing.T) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	t.Helper()
	return testCertificateValidity(t, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
}

func testCertificateValidity(t *testing.T, notBefore, notAfter time.Time) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "family-media.duckdns.org"}, DNSNames: []string{"family-media.duckdns.org"}, NotBefore: notBefore, NotAfter: notAfter}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &certificate, nil }
}
