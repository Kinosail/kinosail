// Package homeassistant provides the local Home Assistant integration protocol.
package homeassistant

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	pairingTTL time.Duration = 600_000_000_000
	playerTTL  time.Duration = 30_000_000_000
	mediaTTL   time.Duration = 300_000_000_000
	maxPairs                 = 32
	maxPlayers               = 64
)

// Profile is the identity state used by the integration.
type Profile[P any] struct {
	Source   P
	ID, Name string
	Owner    bool
	APIKey   bool
	Scopes   []string
}

// Item is one direct media item.
type Item struct{ ID, Path string }

// Library is one Home Assistant browse response.
type Library struct {
	Items                any
	View                 string
	Total, Offset, Limit int
}

// Server identifies one Kinosail server.
type Server struct{ Name, ID string }

// TrustedHTTPS contains the discovery trust state.
type TrustedHTTPS struct {
	Hostname   string
	Configured bool
}

// Config connects the shared protocol to one app.
type Config[P any] struct {
	Lifecycle      context.Context
	AuthURL        string
	Random         io.Reader
	Enabled        func() bool
	SaveEnabled    func(bool) error
	RevokeKeys     func() error
	CreateKey      func(P, string) (string, error)
	FindProfile    func(string) (Profile[P], bool)
	CurrentProfile func(*http.Request) Profile[P]
	Server         func() Server
	TrustedHTTPS   func() TrustedHTTPS
	Browse         func(*http.Request) (Library, error)
	VisibleItem    func(*http.Request, string) (Item, bool)
	FindItem       func(string) (Item, bool)
	SafePath       func(string) bool
}

// Integration owns one Home Assistant protocol lifecycle.
type Integration[P any] struct {
	mu        sync.Mutex
	config    Config[P]
	pairs     map[string]pairing[P]
	players   map[string]playerRecord
	requests  map[string]authorization
	codes     map[string]authorization
	discovery discovery
	advertise advertiseFunc
	secret    [32]byte
	publish   func(string, string, string)
	now       func() time.Time
	text      func() string
}

// New creates one integration and applies its configured startup state.
func New[P any](config Config[P]) (*Integration[P], error) {
	if config.Random == nil {
		config.Random = rand.Reader
	}
	integration := &Integration[P]{
		config: config, pairs: make(map[string]pairing[P]), players: make(map[string]playerRecord),
		requests: make(map[string]authorization), codes: make(map[string]authorization),
		advertise: advertiseHomeAssistant, now: time.Now, text: rand.Text,
	}
	if _, err := io.ReadFull(config.Random, integration.secret[:]); err != nil {
		return nil, fmt.Errorf("create Home Assistant media secret: %w", err)
	}
	if !config.Enabled() {
		if err := config.RevokeKeys(); err != nil {
			return nil, fmt.Errorf("revoke Home Assistant keys: %w", err)
		}
	} else {
		integration.startDiscoveryLocked()
	}
	return integration, nil
}

// Enabled reports the configured feature state.
func (integration *Integration[P]) Enabled() bool { return integration.config.Enabled() }

// SetEnabled applies one complete feature-state transition.
func (integration *Integration[P]) SetEnabled(enabled bool) error {
	integration.mu.Lock()
	defer integration.mu.Unlock()
	if !enabled {
		if err := integration.config.RevokeKeys(); err != nil {
			return err
		}
	}
	if err := integration.config.SaveEnabled(enabled); err != nil {
		return err
	}
	if enabled {
		integration.startDiscoveryLocked()
		return nil
	}
	integration.stopDiscoveryLocked()
	clear(integration.pairs)
	clear(integration.players)
	clear(integration.requests)
	clear(integration.codes)
	return nil
}

// SetPublisher sets the app event adapter.
func (integration *Integration[P]) SetPublisher(publish func(string, string, string)) {
	integration.mu.Lock()
	integration.publish = publish
	integration.mu.Unlock()
}
