package remoteaccess

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveDependenciesDefaultsAndRejectsUnsafeEndpoints(t *testing.T) {
	dependency, endpoint, err := resolveDependencies(nil)
	if err != nil || endpoint.String() != duckDNSUpdateURL || dependency.Client.Timeout != 10*time.Second {
		t.Fatalf("defaults = %#v, %v, %v", dependency, endpoint, err)
	}
	if err = dependency.Client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy = %v", err)
	}
	for _, address := range []string{"%", "http://example.com", "https:///update", "https://user@example.com", "https://example.com?secret=1", "https://example.com/#fragment"} {
		if _, _, err = resolveDependencies([]Dependencies{{UpdateURL: address}}); err == nil {
			t.Fatalf("unsafe endpoint %q accepted", address)
		}
	}
}

func TestNewReportsKillSwitchAndCertificateCacheFailures(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: file}
	if manager, err := New(config); err == nil || manager == nil {
		t.Fatalf("kill-switch read = %#v, %v", manager, err)
	}

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "acme"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	config.DataDir = directory
	if manager, err := New(config); err == nil || manager != nil {
		t.Fatalf("certificate cache = %#v, %v", manager, err)
	}
}

func TestConfigureCertificateValidatesNameAndCertificate(t *testing.T) {
	manager := certificateManager(t, func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return nil, errors.New("issuer unavailable")
	})
	if _, err := manager.certificate(&tls.ClientHelloInfo{ServerName: "attacker.example"}); err == nil {
		t.Fatal("unexpected SNI accepted")
	}
	if _, err := manager.certificate(&tls.ClientHelloInfo{ServerName: "FAMILY.DUCKDNS.ORG."}); err == nil || err.Error() != "issuer unavailable" {
		t.Fatalf("issuer error = %v", err)
	}

	manager = certificateManager(t, nil)
	if manager.certificate == nil {
		t.Fatal("default certificate manager not configured")
	}
}

func TestValidateCertificateRejectsEveryInvalidShape(t *testing.T) {
	manager := &Manager{hostname: "family.duckdns.org", status: Status{State: "starting"}}
	issuerErr := errors.New("issuer unavailable")
	if err := manager.validateCertificate(nil, issuerErr); !errors.Is(err, issuerErr) {
		t.Fatalf("issuer error = %v", err)
	}
	for name, certificate := range map[string]*tls.Certificate{
		"nil":          nil,
		"empty":        {},
		"invalid DER":  {Certificate: [][]byte{[]byte("invalid")}},
		"wrong domain": validLeaf("attacker.example", time.Now().Add(-time.Minute), time.Now().Add(time.Hour)),
		"not active":   validLeaf("family.duckdns.org", time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)),
	} {
		t.Run(name, func(t *testing.T) {
			manager.killed = false
			if err := manager.validateCertificate(certificate, nil); err == nil || manager.status.State != "error" {
				t.Fatalf("invalid certificate = %v, %#v", err, manager.status)
			}
		})
	}
	valid := validLeaf("family.duckdns.org", time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	manager.killed = false
	if err := manager.validateCertificate(valid, nil); err != nil || manager.status.CertificateExpires == "" {
		t.Fatalf("valid certificate = %v, %#v", err, manager.status)
	}
}

func TestKillAndResetRejectUnavailableOrUnremovableMarkers(t *testing.T) {
	disabled, err := New(Config{})
	if err != nil || disabled.Kill() == nil {
		t.Fatalf("disabled kill = %#v, %v", disabled, err)
	}
	manager := activeManager(t)
	if err = os.MkdirAll(manager.killPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(manager.killPath, "keep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = manager.ResetKill(); err == nil {
		t.Fatal("non-empty kill marker directory was removed")
	}
}

func certificateManager(t *testing.T, certificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)) *Manager {
	t.Helper()
	config := Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: t.TempDir()}
	manager := newManager(config, Dependencies{Client: http.DefaultClient, Certificate: certificate}, mustURL(t, duckDNSUpdateURL))
	if err := manager.configureCertificate(); err != nil {
		t.Fatal(err)
	}
	return manager
}

func validLeaf(hostname string, notBefore, notAfter time.Time) *tls.Certificate {
	return &tls.Certificate{Certificate: [][]byte{{1}}, Leaf: &x509.Certificate{DNSNames: []string{hostname}, NotBefore: notBefore, NotAfter: notAfter}}
}

func mustURL(t *testing.T, address string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
