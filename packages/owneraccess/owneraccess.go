// Package owneraccess provides a private, device-bound management tunnel.
// Its userspace network has no route to the host, the LAN, or other peers.
package owneraccess

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const (
	Address    = "10.92.0.1"
	Port       = 51821
	maxDevices = 32
)

type Device struct {
	Label        string `json:"label"`
	ProfileID    string `json:"profileId"`
	Revision     uint64 `json:"revision"`
	PublicKey    string `json:"publicKey"`
	Address      string `json:"address"`
	PresharedKey string `json:"presharedKey,omitempty"`
}

type state struct {
	Enabled    bool     `json:"enabled"`
	Endpoint   string   `json:"endpoint"`
	PrivateKey string   `json:"privateKey"`
	Devices    []Device `json:"devices"`
}

type Config struct {
	Directory        string
	Origin           string
	Profile          func(string) (identitycore.Profile, bool)
	Certificate      func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	MaintainEndpoint func(context.Context, string) error
}

type Status struct {
	Enabled  bool     `json:"enabled"`
	State    string   `json:"state"`
	URL      string   `json:"url,omitempty"`
	Endpoint string   `json:"endpoint,omitempty"`
	Devices  []Device `json:"devices"`
	Warning  string   `json:"warning,omitempty"`
}

type Manager struct {
	mu            sync.Mutex
	config        Config
	state         state
	runtime       *runtime
	handler       http.Handler
	ctx           context.Context
	err           error
	write         func(string, []byte) error
	retry         bool
	endpointError bool
}

// Open reads private state. It neither enables access nor opens a network port.
func Open(config Config) (*Manager, error) {
	if !filepath.IsAbs(config.Directory) || filepath.Clean(config.Directory) != config.Directory || config.Profile == nil || config.Certificate == nil {
		return nil, errors.New("private management configuration is invalid")
	}
	if validOrigin(config.Origin) {
		u, _ := url.Parse(config.Origin)
		if u.Port() == "443" {
			u.Host = u.Hostname()
			config.Origin = u.String()
		}
	}
	m := &Manager{config: config, write: privatefile.Write}
	if _, err := os.Lstat(filepath.Join(config.Directory, "owner-access.disabled")); err == nil {
		return m, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("private management recovery state is unavailable")
	}
	data, err := privatefile.Read(filepath.Join(config.Directory, "owner-access.json"), 64<<10)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, errors.New("private management state is unavailable")
	}
	if httpguard.DecodeUniqueJSON(bytes.NewReader(data), 64<<10, &m.state) != nil || !validState(m.state) {
		return nil, errors.New("private management state is invalid")
	}
	return m, nil
}

// Attach supplies the existing authenticated application, then resumes explicitly enabled access.
func (m *Manager) Attach(ctx context.Context, handler http.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx, m.handler = ctx, handler
	m.retry = true
	if m.state.Enabled {
		m.err = m.startLocked()
	}
	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.stopLocked()
	}()
	go m.retryStartup(ctx)
}

// Enable requires a configured HTTPS identity and an explicit, bounded public endpoint.
func (m *Manager) Enable(endpoint string) error {
	if !validEndpoint(endpoint) || !validOrigin(m.config.Origin) {
		return errors.New("set up trusted HTTPS and enter a hostname with a UDP port between 1024 and 65535")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Enabled {
		return errors.New("turn management off before changing its endpoint")
	}
	if m.ctx == nil || m.ctx.Err() != nil || m.handler == nil {
		return errors.New("private management is unavailable")
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return errors.New("could not create private management identity")
	}
	next := state{Enabled: true, Endpoint: endpoint, PrivateKey: base64.StdEncoding.EncodeToString(key.Bytes())}
	previous := m.state
	m.state = next
	if err = m.startLocked(); err != nil {
		m.state = previous
		return err
	}
	if err = m.save(next); err != nil {
		m.stopLocked()
		m.state = previous
		_ = privatefile.Create(filepath.Join(m.config.Directory, "owner-access.disabled"))
		return errors.New("could not save management settings")
	}
	if err = privatefile.Remove(filepath.Join(m.config.Directory, "owner-access.disabled")); err != nil {
		m.stopLocked()
		return errors.New("could not clear management recovery lock")
	}
	m.err = nil
	m.retry = true
	m.endpointError = false
	return nil
}

// Disable closes active tunnels before durably clearing every pairing and the Server key.
func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
	m.retry = false
	m.state = state{}
	m.endpointError = false
	markerErr := privatefile.Create(filepath.Join(m.config.Directory, "owner-access.disabled"))
	m.err = m.save(m.state)
	if markerErr != nil && !errors.Is(markerErr, os.ErrExist) {
		m.err = markerErr
	}
	if m.err != nil {
		return errors.New("management is off, but saving failed; stop the Server before restarting it")
	}
	return nil
}

// Pair binds one device to the signed-in Owner. Keys leave the Server only in this response.
func (m *Manager) Pair(label, profileID string) (string, error) {
	label = strings.TrimSpace(label)
	if !validLabel(label) || !validID(profileID) {
		return "", errors.New("enter a device name of 1 to 80 characters")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.config.Profile(profileID)
	if !ok || !eligible(p) || p.ID != profileID {
		return "", errors.New("pairing requires an active Owner with two-step sign-in")
	}
	if !m.state.Enabled || m.runtime == nil || len(m.state.Devices) >= maxDevices {
		return "", errors.New("enable management first, or remove an unused device")
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", errors.New("could not create device identity")
	}
	psk := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, psk); err != nil {
		return "", errors.New("could not create device identity")
	}
	// Addresses are never reused during this runtime: an old TCP connection cannot become a new device.
	address := m.runtime.nextAddress()
	if address == "" {
		return "", errors.New("restart private management before pairing more devices")
	}
	peer := Device{label, profileID, p.Revision, base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), address, base64.StdEncoding.EncodeToString(psk)}
	next := m.state
	next.Devices = append(append([]Device(nil), m.state.Devices...), peer)
	if err = m.save(next); err != nil {
		return "", errors.New("could not save paired device")
	}
	m.state = next
	if err = m.runtime.add(peer); err != nil {
		m.stopLocked()
		m.retry = false
		m.err = err
		return "", errors.New("management stopped; turn it off before pairing again")
	}
	serverKey, _ := ecdh.X25519().NewPrivateKey(decodeKey(m.state.PrivateKey))
	return fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = %s\n\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nEndpoint = %s\nAllowedIPs = %s/32\nPersistentKeepalive = 25\n", base64.StdEncoding.EncodeToString(key.Bytes()), address, Address, base64.StdEncoding.EncodeToString(serverKey.PublicKey().Bytes()), peer.PresharedKey, m.state.Endpoint, Address), nil
}

func (m *Manager) Revoke(key string) error {
	if decodeKey(key) == nil {
		return errors.New("paired device not found")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, peer := range m.state.Devices {
		if peer.PublicKey != key {
			continue
		}
		next := m.state
		next.Devices = append(append([]Device(nil), next.Devices[:i]...), next.Devices[i+1:]...)
		// Stop accepting the peer even if durable storage fails.
		if m.runtime != nil {
			m.runtime.remove(peer)
		}
		m.state = next
		if err := m.save(next); err != nil {
			m.stopLocked()
			m.retry = false
			_ = privatefile.Create(filepath.Join(m.config.Directory, "owner-access.disabled"))
			m.err = err
			return errors.New("management stopped because device revocation could not be saved; stop the Server before restarting")
		}
		return nil
	}
	return errors.New("paired device not found")
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := Status{Enabled: m.state.Enabled, State: "off", Devices: []Device{}}
	if m.state.Enabled {
		result.State, result.URL, result.Endpoint = "unavailable", m.config.Origin, m.state.Endpoint
	}
	if m.runtime != nil {
		result.State = "ready"
	}
	if m.endpointError {
		result.Warning = "Your public hostname could not be refreshed. Check its DNS settings before leaving home."
	}
	for _, peer := range m.state.Devices {
		peer.PresharedKey = ""
		result.Devices = append(result.Devices, peer)
	}
	return result
}

func (m *Manager) save(value state) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return m.write(filepath.Join(m.config.Directory, "owner-access.json"), data)
}
