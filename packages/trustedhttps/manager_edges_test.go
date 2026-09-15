package trustedhttps

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerDefaultDependenciesAndNilReceiver(t *testing.T) {
	manager, err := New(validIssuerConfig(), t.TempDir())
	if err != nil || manager.client == nil || manager.now == nil || manager.obtain == nil || manager.wait == nil {
		t.Fatalf("default manager = %#v, %v", manager, err)
	}
	var absent *Manager
	absent.Run(t.Context())
	if absent.Status().State != "disabled" {
		t.Fatalf("nil status = %#v", absent.Status())
	}
}

func TestManagerMaintenanceReportsObtainWriteAndLoadFailures(t *testing.T) {
	for name, setup := range map[string]func(*Manager){
		"obtain": func(manager *Manager) {
			manager.obtain = func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
				return nil, errors.New("obtain failed")
			}
		},
		"write": func(manager *Manager) {
			blocked := filepath.Join(t.TempDir(), "blocked")
			if err := os.WriteFile(blocked, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			manager.dataDir = blocked
			manager.obtain = invalidIdentity
		},
		"load": func(manager *Manager) {
			manager.obtain = invalidIdentity
		},
	} {
		t.Run(name, func(t *testing.T) {
			manager, err := New(validIssuerConfig(), t.TempDir(), Dependencies{Client: defaultHTTPClient()})
			if err != nil {
				t.Fatal(err)
			}
			manager.provider = &issuerProvider{}
			setup(manager)
			if err = manager.maintain(t.Context()); err == nil {
				t.Fatal("maintenance failure accepted")
			}
		})
	}
}

func TestRunUsesRetryDelayAfterMaintenanceFailure(t *testing.T) {
	manager, err := New(validIssuerConfig(), t.TempDir(), Dependencies{Client: defaultHTTPClient()})
	if err != nil {
		t.Fatal(err)
	}
	manager.provider = failingDNSProvider{}
	waits := make([]time.Duration, 0, 2)
	manager.wait = func(_ context.Context, delay time.Duration) bool {
		waits = append(waits, delay)
		return len(waits) == 1
	}
	manager.Run(t.Context())
	if len(waits) != 2 || waits[0] != time.Hour || waits[1] != time.Hour || manager.Status().State != "error" {
		t.Fatalf("retry waits = %v, status=%#v", waits, manager.Status())
	}
}

func TestLoadCertificateRejectsInvalidLeaf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.pem")
	contents := testIdentity(t, validIssuerConfig().Hostname(), time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadCertificate(path, validIssuerConfig().Hostname(), time.Now()); err == nil {
		t.Fatal("expired cached identity accepted")
	}
}

func invalidIdentity(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error) {
	return []byte("invalid"), nil
}

type failingDNSProvider struct{}

func (failingDNSProvider) UpdateAddress(context.Context) error      { return errors.New("offline") }
func (failingDNSProvider) PresentTXT(context.Context, string) error { return nil }
func (failingDNSProvider) CleanupTXT(context.Context) error         { return nil }
