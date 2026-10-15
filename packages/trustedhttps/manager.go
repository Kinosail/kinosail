package trustedhttps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const certificateName = "tls-duckdns.pem"

type Status struct {
	State              string `json:"state"`
	Provider           string `json:"provider,omitempty"`
	Hostname           string `json:"hostname,omitempty"`
	Address            string `json:"address,omitempty"`
	CertificateExpires string `json:"certificateExpires,omitempty"`
	Error              string `json:"error,omitempty"`
}

type Dependencies struct {
	Client       *http.Client
	UpdateURL    string
	DeSECURL     string
	Now          func() time.Time
	obtain       func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error)
	Wait         func(context.Context, time.Duration) bool
	DirectoryURL string
}

type Manager struct {
	certificateOnly bool
	config          Config
	dataDir         string
	client          *http.Client
	provider        dnsProvider
	now             func() time.Time
	obtain          func(context.Context, Config, string, *http.Client, string, dnsProvider) ([]byte, error)
	wait            func(context.Context, time.Duration) bool
	directory       string
	mu              sync.RWMutex
	status          Status
	cert            *tls.Certificate
	leaf            *x509.Certificate
}

func New(config Config, dataDir string, dependencies ...Dependencies) (*Manager, error) {
	if len(dependencies) > 1 {
		return nil, errors.New("trusted HTTPS accepts at most one dependency set")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config == (Config{}) {
		return &Manager{status: Status{State: "disabled"}}, nil
	}
	if dataDir == "" {
		return nil, errors.New("trusted HTTPS data directory is required")
	}
	dependency := Dependencies{}
	if len(dependencies) != 0 {
		dependency = dependencies[0]
	}
	if dependency.Client == nil {
		dependency.Client = defaultHTTPClient()
	}
	provider, err := newDNSProvider(config, dependency.Client, providerEndpoints{duckDNS: dependency.UpdateURL, deSEC: dependency.DeSECURL})
	if err != nil {
		return nil, err
	}
	if dependency.Now == nil {
		dependency.Now = time.Now
	}
	if dependency.obtain == nil {
		dependency.obtain = obtainCertificate
	}
	if dependency.Wait == nil {
		dependency.Wait = waitContext
	}
	manager := &Manager{config: config, dataDir: dataDir, client: dependency.Client, provider: provider, now: dependency.Now, obtain: dependency.obtain, wait: dependency.Wait, directory: dependency.DirectoryURL, status: Status{State: "starting", Provider: config.ProviderName(), Hostname: config.Hostname(), Address: config.Address}}
	certificate, leaf, err := loadCertificate(filepath.Join(dataDir, certificateName), config.Hostname(), dependency.Now())
	if err == nil {
		manager.cert, manager.leaf = certificate, leaf
		manager.status.State, manager.status.CertificateExpires = "ready", leaf.NotAfter.UTC().Format(time.RFC3339)
	}
	slog.Info("trusted HTTPS manager initialized", "provider", config.ProviderName(), "hostname", config.Hostname(), "state", manager.status.State)
	return manager, nil
}

func (manager *Manager) Run(ctx context.Context) {
	if manager == nil || manager.config == (Config{}) {
		return
	}
	for {
		delay := 12 * time.Hour
		if err := manager.maintain(ctx); err != nil {
			manager.setError(err)
			delay = time.Hour
		}
		if !manager.wait(ctx, delay) {
			return
		}
	}
}

func (manager *Manager) Certificate(serverName string) *tls.Certificate {
	if manager == nil || !strings.EqualFold(strings.TrimSuffix(serverName, "."), manager.config.Hostname()) {
		return nil
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.cert
}

func (manager *Manager) Status() Status {
	if manager == nil {
		return Status{State: "disabled"}
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.status
}

func (manager *Manager) maintain(ctx context.Context) error {
	if !manager.certificateOnly {
		slog.Info("trusted HTTPS address update started", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname(), "address", manager.config.Address)
		if err := manager.provider.UpdateAddress(ctx); err != nil {
			return err
		}
		slog.Info("trusted HTTPS address update complete", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname())
	}
	manager.mu.RLock()
	leaf := manager.leaf
	manager.mu.RUnlock()
	if leaf != nil && manager.now().Add(30*24*time.Hour).Before(leaf.NotAfter) {
		manager.setReady(leaf)
		slog.Info("trusted HTTPS certificate remains valid", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname(), "expires", leaf.NotAfter.UTC().Format(time.RFC3339))
		return nil
	}
	slog.Info("trusted HTTPS certificate renewal started", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname())
	contents, err := manager.obtain(ctx, manager.config, manager.dataDir, manager.client, manager.directory, manager.provider)
	if err != nil {
		return err
	}
	path := filepath.Join(manager.dataDir, certificateName)
	if err = writePrivate(path, contents); err != nil {
		return errors.New("save trusted HTTPS certificate")
	}
	certificate, renewed, err := loadCertificate(path, manager.config.Hostname(), manager.now())
	if err != nil {
		return err
	}
	manager.mu.Lock()
	manager.cert, manager.leaf = certificate, renewed
	manager.status.State, manager.status.CertificateExpires, manager.status.Error = "ready", renewed.NotAfter.UTC().Format(time.RFC3339), ""
	manager.mu.Unlock()
	slog.Info("trusted HTTPS certificate ready", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname(), "expires", renewed.NotAfter.UTC().Format(time.RFC3339))
	return nil
}

func (manager *Manager) setReady(leaf *x509.Certificate) {
	manager.mu.Lock()
	manager.status.State, manager.status.CertificateExpires, manager.status.Error = "ready", leaf.NotAfter.UTC().Format(time.RFC3339), ""
	manager.mu.Unlock()
}

func (manager *Manager) setError(err error) {
	manager.mu.Lock()
	manager.status.State, manager.status.Error = "error", err.Error()
	manager.mu.Unlock()
	slog.Warn("trusted HTTPS maintenance failed", "provider", manager.config.ProviderName(), "hostname", manager.config.Hostname(), "error", err)
}

func loadCertificate(path, hostname string, now time.Time) (*tls.Certificate, *x509.Certificate, error) {
	contents, err := readBoundedFile(path, 256<<10)
	if err != nil {
		return nil, nil, err
	}
	certificate, err := tls.X509KeyPair(contents, contents)
	if err != nil || len(certificate.Certificate) == 0 {
		return nil, nil, errors.New("trusted HTTPS certificate is invalid")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || leaf.VerifyHostname(hostname) != nil || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, nil, errors.New("trusted HTTPS certificate is invalid")
	}
	certificate.Leaf = leaf
	return &certificate, leaf, nil
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
