// Package remoteaccess exposes one optional, direct HTTPS listener backed by DuckDNS.
package remoteaccess

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/privatefile"
	"golang.org/x/crypto/acme/autocert"
)

const duckDNSUpdateURL = "https://www.duckdns.org/update"

// Config is the complete interface for secure public access.
type Config struct {
	Enabled, PublicHTTPS  bool
	Gateway               bool
	Domain, Token, Listen string
	DataDir               string
}

// Dependencies contains internal seams used to verify network and certificate behavior.
type Dependencies struct {
	Client      *http.Client
	UpdateURL   string
	Certificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

// Status contains no credentials and is safe to expose to an Owner.
type Status struct {
	Isolation          string `json:"isolation,omitempty"`
	State              string `json:"state"`
	Mode               string `json:"mode"`
	Hostname           string `json:"hostname,omitempty"`
	Policy             string `json:"policy,omitempty"`
	LastDDNSUpdate     string `json:"lastDdnsUpdate,omitempty"`
	CertificateExpires string `json:"certificateExpires,omitempty"`
	Connections        int    `json:"connections,omitempty"`
	Error              string `json:"error,omitempty"`
}

// Manager owns dynamic DNS, public certificates, and the dedicated listener.
type Manager struct {
	config      Config
	client      *http.Client
	updateURL   string
	certificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	control     sync.Mutex // Serializes kill/reset and their durable marker changes.
	mu          sync.RWMutex
	status      Status
	server      *http.Server
	listener    net.Listener
	connections map[net.Conn]string
	sources     map[string]int
	operations  transportOperations
	killed      bool
	killPath    string
	hostname    string
}

// New validates all input before creating files or making network requests.
func New(config Config, dependencies ...Dependencies) (*Manager, error) {
	if !config.Enabled {
		return &Manager{config: config, status: Status{State: "disabled", Mode: "off"}}, nil
	}
	var err error
	if config, err = validateConfig(config); err != nil {
		return nil, err
	}
	dependency, endpoint, err := resolveDependencies(dependencies)
	if err != nil {
		return nil, err
	}
	manager := newManager(config, dependency, endpoint)
	if stopped, err := manager.readKillSwitch(); stopped || err != nil {
		return manager, err
	}
	if err := manager.configureCertificate(); err != nil {
		return nil, err
	}
	return manager, nil
}

func validateConfig(config Config) (Config, error) {
	if config.Gateway && !config.PublicHTTPS {
		return Config{}, errors.New("the public gateway requires HTTPS mode")
	}
	config.Domain = strings.ToLower(strings.TrimSpace(config.Domain))
	if !validDomain(config.Domain) {
		return Config{}, errors.New("DuckDNS domain must be one subdomain label")
	}
	if !validToken(config.Token) {
		return Config{}, errors.New("DuckDNS token must contain 32 to 128 safe characters")
	}
	if config.PublicHTTPS {
		if config.DataDir == "" {
			return Config{}, errors.New("remote access data directory is required")
		}
		if !validListen(config.Listen) {
			return Config{}, errors.New("remote access listen address is invalid")
		}
	}
	return config, nil
}

func resolveDependencies(dependencies []Dependencies) (Dependencies, *url.URL, error) {
	if len(dependencies) > 1 {
		return Dependencies{}, nil, errors.New("remote access accepts at most one dependency set")
	}
	dependency := Dependencies{}
	if len(dependencies) != 0 {
		dependency = dependencies[0]
	}
	if dependency.Client == nil {
		dependency.Client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if dependency.UpdateURL == "" {
		dependency.UpdateURL = duckDNSUpdateURL
	}
	endpoint, err := url.Parse(dependency.UpdateURL)
	if err != nil || !validUpdateEndpoint(endpoint) {
		return Dependencies{}, nil, errors.New("DuckDNS update address must be an HTTPS URL")
	}
	return dependency, endpoint, nil
}

func validUpdateEndpoint(endpoint *url.URL) bool {
	return endpoint.Scheme == "https" && endpoint.Host != "" && endpoint.User == nil && endpoint.RawQuery == "" && endpoint.Fragment == ""
}

func newManager(config Config, dependency Dependencies, endpoint *url.URL) *Manager {
	hostname := config.Domain + ".duckdns.org"
	status := Status{State: "starting", Mode: "https", Hostname: hostname, Policy: "public-v1"}
	if !config.PublicHTTPS {
		status.Mode, status.Policy = "dns-only", ""
	}
	return &Manager{config: config, client: dependency.Client, updateURL: endpoint.String(), certificate: dependency.Certificate, hostname: hostname, status: status, connections: make(map[net.Conn]string), sources: make(map[string]int), operations: defaultTransportOperations()}
}

func (manager *Manager) readKillSwitch() (bool, error) {
	if !manager.config.PublicHTTPS {
		return false, nil
	}
	manager.killPath = filepath.Join(manager.config.DataDir, "remote-access.disabled")
	if _, err := os.Stat(manager.killPath); err == nil {
		manager.killed, manager.status.State = true, "killed"
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, errors.New("read remote access kill switch")
	}
	return false, nil
}

func (manager *Manager) configureCertificate() error {
	if !manager.config.PublicHTTPS {
		return nil
	}
	if manager.certificate == nil {
		cache := filepath.Join(manager.config.DataDir, "acme")
		if err := os.MkdirAll(cache, 0o700); err != nil {
			return fmt.Errorf("create certificate cache: %w", err)
		}
		certificateManager := &autocert.Manager{Cache: autocert.DirCache(cache), Prompt: autocert.AcceptTOS, HostPolicy: autocert.HostWhitelist(manager.hostname)}
		manager.certificate = certificateManager.GetCertificate
	}
	certificate := manager.certificate
	manager.certificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if !strings.EqualFold(strings.TrimSuffix(hello.ServerName, "."), manager.hostname) {
			return nil, errors.New("public TLS name is not allowed")
		}
		result, err := certificate(hello)
		return result, manager.validateCertificate(result, err)
	}
	return nil
}

func (manager *Manager) validateCertificate(result *tls.Certificate, err error) error {
	if err != nil {
		return err
	}
	if result == nil || len(result.Certificate) == 0 {
		return manager.certificateError()
	}
	leaf := result.Leaf
	if leaf == nil {
		leaf, _ = x509.ParseCertificate(result.Certificate[0])
	}
	if leaf == nil {
		return manager.certificateError()
	}
	manager.mu.Lock()
	manager.status.CertificateExpires = leaf.NotAfter.UTC().Format(time.RFC3339)
	manager.mu.Unlock()
	now := time.Now()
	if leaf.VerifyHostname(manager.hostname) != nil || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return manager.certificateError()
	}
	return nil
}

func (manager *Manager) certificateError() error {
	err := errors.New("public certificate is invalid")
	manager.setStatus("error", err)
	return err
}

// Kill closes the dedicated public listener immediately and persists fail-closed state.
func (manager *Manager) Kill() error { //nolint:cyclop // Every resource is closed even when persistence fails.
	manager.control.Lock()
	defer manager.control.Unlock()
	if !manager.config.Enabled || !manager.config.PublicHTTPS || manager.killPath == "" {
		return errors.New("public HTTPS remote access is not enabled")
	}
	persistErr := privatefile.Create(manager.killPath)
	manager.mu.Lock()
	manager.killed, manager.status.State, manager.status.Error = true, "killed", ""
	server, listener := manager.server, manager.listener
	connections := make([]net.Conn, 0, len(manager.connections))
	for connection := range manager.connections {
		connections = append(connections, connection)
	}
	clear(manager.connections)
	clear(manager.sources)
	manager.status.Connections = 0
	manager.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	if server != nil {
		_ = server.Close()
	}
	for _, connection := range connections {
		_ = connection.Close()
	}
	if persistErr != nil && !os.IsExist(persistErr) {
		return errors.New("public access is off but the kill switch could not be persisted")
	}
	return nil
}

// ResetKill allows public HTTPS to start again after the local Server restarts.
func (manager *Manager) ResetKill() error {
	manager.control.Lock()
	defer manager.control.Unlock()
	if !manager.config.Enabled || !manager.config.PublicHTTPS || manager.killPath == "" {
		return errors.New("public HTTPS remote access is not enabled")
	}
	manager.mu.RLock()
	killed := manager.killed
	manager.mu.RUnlock()
	if !killed {
		return errors.New("public access must be stopped before it can be re-enabled")
	}
	if err := os.Remove(manager.killPath); err != nil && !os.IsNotExist(err) {
		return errors.New("reset remote access kill switch")
	}
	manager.mu.Lock()
	// An in-flight Serve retry must stay stopped until a new Manager is created.
	manager.killed, manager.status.State, manager.status.Error = true, "restart-required", ""
	manager.mu.Unlock()
	return nil
}

// Status reports current readiness without secrets.
func (manager *Manager) Status() Status {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.status
}

func (manager *Manager) setStatus(state string, err error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.killed {
		return
	}
	manager.status.State, manager.status.Error = state, ""
	if err != nil {
		manager.status.Error = err.Error()
	}
}
