// Package wireguard creates direct, independently paired WireGuard profiles.
package wireguard

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/privatefile"
	"golang.org/x/crypto/curve25519"
)

// Viewer is a locally authorized WireGuard peer without its one-time private key.
type Viewer struct {
	Label        string `json:"label"`
	ProfileID    string `json:"profileId"`
	PublicKey    string `json:"publicKey"`
	Address      string `json:"address"`
	PresharedKey string `json:"presharedKey,omitempty"`
}

type state struct {
	PrivateKey string   `json:"privateKey"`
	PublicKey  string   `json:"publicKey"`
	Peers      []Viewer `json:"peers"`
}

// Manager retains the Server's WireGuard identity and authorized peers locally.
type Manager struct {
	mu        sync.Mutex
	directory string
	endpoint  string
	state     state
	deps      dependencies
}

type dependencies struct {
	keypair   func() (string, string, error)
	randomKey func() (string, error)
	mkdirAll  func(string, os.FileMode) error
	chmod     func(string, os.FileMode) error
	marshal   func(state) ([]byte, error)
	write     func(string, []byte) error
}

// Pairing contains the current Server configuration and a one-time Viewer profile.
type Pairing struct {
	ServerConfig    string
	ViewerConfig    string
	ServerPublicKey string
	ViewerPublicKey string
}

// Open loads or creates local WireGuard pairing state.
func Open(directory, endpoint string) (*Manager, error) { //nolint:cyclop // Strict persisted-state validation must finish before returning a Manager.
	return open(directory, endpoint, defaultDependencies())
}

func open(directory, endpoint string, deps dependencies) (*Manager, error) { //nolint:cyclop // Strict persisted-state validation must finish before returning a Manager.
	if directory == "" || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, errors.New("WireGuard directory must be an absolute clean path")
	}
	if !validEndpoint(endpoint) {
		return nil, errors.New("WireGuard endpoint must be a host on port 51820")
	}
	manager := &Manager{directory: directory, endpoint: endpoint, deps: deps}
	data, err := privatefile.Read(filepath.Join(directory, "state.json"), 256<<10)
	if errors.Is(err, os.ErrNotExist) {
		manager.state.PrivateKey, manager.state.PublicKey, err = deps.keypair()
		if err == nil {
			err = manager.save()
		}
		return manager, err
	}
	if err != nil {
		return nil, errors.New("invalid WireGuard pairing state")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manager.state) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validState(manager.state) {
		return nil, errors.New("invalid WireGuard pairing state")
	}
	return manager, nil
}

func validEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	return err == nil && host != "" && len(host) <= 253 && port == "51820" && !strings.ContainsAny(host, " /?#@") && strings.IndexFunc(host, unicode.IsControl) == -1
}

// Pair authorizes one Viewer and returns its private profile only to the local caller.
func (manager *Manager) Pair(label, profileID string) (Pairing, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	label = strings.TrimSpace(label)
	if !validLabel(label) || !validProfileID(profileID) || len(manager.state.Peers) >= 253 {
		return Pairing{}, errors.New("Viewer label is invalid or no addresses remain")
	}
	viewerPrivate, viewerPublic, err := manager.deps.keypair()
	if err != nil {
		return Pairing{}, err
	}
	presharedKey, err := manager.deps.randomKey()
	if err != nil {
		return Pairing{}, err
	}
	address := manager.nextAddress()
	manager.state.Peers = append(manager.state.Peers, Viewer{Label: label, ProfileID: profileID, PublicKey: viewerPublic, Address: address, PresharedKey: presharedKey})
	if err := manager.save(); err != nil {
		manager.state.Peers = manager.state.Peers[:len(manager.state.Peers)-1]
		return Pairing{}, err
	}
	return Pairing{
		ServerConfig: manager.serverConfig(), ServerPublicKey: manager.state.PublicKey, ViewerPublicKey: viewerPublic,
		ViewerConfig: fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\n\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nEndpoint = %s\nAllowedIPs = 10.91.0.1/32\nPersistentKeepalive = 25\n", viewerPrivate, address, manager.state.PublicKey, presharedKey, manager.endpoint),
	}, nil
}

// Peers returns the authorized Viewers without private keys.
func (manager *Manager) Peers() []Viewer {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	peers := append([]Viewer(nil), manager.state.Peers...)
	for index := range peers {
		peers[index].PresharedKey = ""
	}
	return peers
}

// Revoke removes one Viewer public key from the WireGuard Server configuration.
func (manager *Manager) Revoke(publicKey string) error {
	if !validKey(publicKey) {
		return errors.New("WireGuard Viewer not found")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for index, viewer := range manager.state.Peers {
		if viewer.PublicKey != publicKey {
			continue
		}
		peers := append([]Viewer(nil), manager.state.Peers...)
		manager.state.Peers = append(manager.state.Peers[:index], manager.state.Peers[index+1:]...)
		if err := manager.save(); err != nil {
			manager.state.Peers = peers
			return err
		}
		return nil
	}
	return errors.New("WireGuard Viewer not found")
}

func (manager *Manager) nextAddress() string {
	for octet := 2; octet < 255; octet++ {
		candidate := fmt.Sprintf("10.91.0.%d", octet)
		used := false
		for _, viewer := range manager.state.Peers {
			used = used || viewer.Address == candidate
		}
		if !used {
			return candidate
		}
	}
	return ""
}

func keypair(random io.Reader) (string, string, error) {
	private := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(random, private); err != nil {
		return "", "", err
	}
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64
	public, err := curve25519.X25519(private, curve25519.Basepoint)
	return base64.StdEncoding.EncodeToString(private), base64.StdEncoding.EncodeToString(public), err
}

func randomKey(random io.Reader) (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

func (manager *Manager) serverConfig() string {
	var config strings.Builder
	fmt.Fprintf(&config, "[Interface]\nPrivateKey = %s\nAddress = 10.91.0.1/24\nListenPort = 51820\nSaveConfig = false\n", manager.state.PrivateKey)
	for _, peer := range manager.state.Peers {
		fmt.Fprintf(&config, "\n[Peer]\n# %s\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s/32\n", peer.Label, peer.PublicKey, peer.PresharedKey, peer.Address)
	}
	return config.String()
}

func (manager *Manager) save() error {
	configurationDirectory := filepath.Join(manager.directory, "wg_confs")
	if err := manager.deps.mkdirAll(configurationDirectory, 0o700); err != nil {
		return err
	}
	if err := manager.deps.chmod(manager.directory, 0o700); err != nil { //nolint:gosec // Directories require execute permission; 0700 is owner-only.
		return err
	}
	if err := manager.deps.chmod(configurationDirectory, 0o700); err != nil { //nolint:gosec // Directories require execute permission; 0700 is owner-only.
		return err
	}
	data, err := manager.deps.marshal(manager.state) //nolint:gosec // G117: the local WireGuard private key must persist on the owner-hosted Server.
	if err != nil {
		return err
	}
	if err := manager.deps.write(filepath.Join(manager.directory, "wg_confs", "kinosail.conf"), []byte(manager.serverConfig())); err != nil {
		return err
	}
	return manager.deps.write(filepath.Join(manager.directory, "state.json"), data)
}

func defaultDependencies() dependencies {
	return dependencies{
		keypair:   func() (string, string, error) { return keypair(rand.Reader) },
		randomKey: func() (string, error) { return randomKey(rand.Reader) },
		mkdirAll:  os.MkdirAll,
		chmod:     os.Chmod,
		marshal:   func(value state) ([]byte, error) { return json.Marshal(value) }, //nolint:gosec // G117: the owner-hosted state must persist the local WireGuard private key.
		write:     privatefile.Write,
	}
}

func validState(value state) bool { //nolint:cyclop // Every key, address, label, and uniqueness invariant is checked together before use.
	private, privateOK := decodeKey(value.PrivateKey)
	public, publicOK := decodeKey(value.PublicKey)
	derived, err := curve25519.X25519(private, curve25519.Basepoint)
	if !privateOK || !publicOK || err != nil || subtle.ConstantTimeCompare(derived, public) != 1 || len(value.Peers) > 253 {
		return false
	}
	keys, presharedKeys, addresses := make(map[string]bool, len(value.Peers)), make(map[string]bool, len(value.Peers)), make(map[string]bool, len(value.Peers))
	for _, peer := range value.Peers {
		address := net.ParseIP(peer.Address).To4()
		if !validLabel(peer.Label) || !validProfileID(peer.ProfileID) || !validKey(peer.PublicKey) || !validKey(peer.PresharedKey) || peer.PublicKey == value.PublicKey || address == nil || address[0] != 10 || address[1] != 91 || address[2] != 0 || address[3] < 2 || keys[peer.PublicKey] || presharedKeys[peer.PresharedKey] || addresses[peer.Address] {
			return false
		}
		keys[peer.PublicKey], presharedKeys[peer.PresharedKey], addresses[peer.Address] = true, true, true
	}
	return true
}

func validKey(value string) bool {
	_, valid := decodeKey(value)
	return valid
}

func decodeKey(value string) ([]byte, bool) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	return decoded, err == nil && len(decoded) == curve25519.ScalarSize && base64.StdEncoding.EncodeToString(decoded) == value
}

func validLabel(label string) bool {
	if label = strings.TrimSpace(label); label == "" || len(label) > 80 || !utf8.ValidString(label) {
		return false
	}
	for _, character := range label {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validProfileID(profileID string) bool {
	if profileID = strings.TrimSpace(profileID); profileID == "" || len(profileID) > 128 || !utf8.ValidString(profileID) {
		return false
	}
	for _, character := range profileID {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}
