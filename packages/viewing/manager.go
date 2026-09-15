package viewing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

const (
	previewTTL     = time.Duration(600_000_000_000)
	previewLimit   = 100
	previewWorkers = 2
	syncWorkers    = 2
)

// Profile is the minimum destination identity used by viewing import policy.
type Profile struct {
	ID, Name string
	Owner    bool
}

// Config supplies app-private storage and profile adapters to one Manager.
type Config struct {
	Client         *http.Client
	Persist        func(string, any) error
	Snapshot       func() ([]library.Item, error)
	Profile        func(string) (Profile, bool)
	Progress       func(Profile, string) catalog.PlaybackState
	ListAdditions  func(Profile, string, bool, map[string]int) int
	ImportProgress func(Profile, []ProgressChange) (int, int, error)
	Commit         func(Profile, []ProgressChange, []ListChange) (int, int, int, error)
}

// Manager owns preview, source, persistence, and recurring-sync lifecycles.
type Manager struct {
	mu           sync.Mutex
	file         string
	config       Config
	previews     map[string]Preview
	previewSlots chan struct{}
	syncs        map[string]Sync
	syncSlots    chan struct{}
	syncDeferred atomic.Uint64
	loadErr      error
	engine       Engine[Profile]
}

// NewManager restores durable sync state and starts its bounded scheduler.
func NewManager(ctx context.Context, dataDir string, config Config) *Manager {
	manager := &Manager{
		file: filepath.Join(dataDir, "viewing_imports.json"), config: config,
		previews: make(map[string]Preview), previewSlots: make(chan struct{}, previewWorkers),
		syncs: make(map[string]Sync), syncSlots: make(chan struct{}, syncWorkers),
	}
	if dataDir == "" {
		manager.file = ""
	}
	manager.engine = Engine[Profile]{
		Store: PreviewStore{Mutex: &manager.mu, Values: &manager.previews, Slots: &manager.previewSlots, Limit: previewLimit, Workers: previewWorkers},
		TTL:   previewTTL, Validate: manager.validate, Fetch: func(ctx context.Context, input Input) ([]Activity, error) {
			return Fetch(ctx, manager.config.Client, input, true)
		}, Snapshot: config.Snapshot, Profile: config.Profile,
		ProfileID: func(profile Profile) string { return profile.ID }, ProfileName: func(profile Profile) string { return profile.Name },
		Progress: config.Progress, ListAdditions: config.ListAdditions, Commit: config.Commit,
	}
	if manager.file != "" {
		manager.syncs, manager.loadErr = LoadSyncs(manager.file, func(input Input) (Input, error) {
			input, _, err := manager.validate(input)
			return input, err
		})
	}
	if ctx != nil {
		go manager.schedule(ctx)
	}
	return manager
}

// Preview validates and fetches one credential-safe import plan.
func (manager *Manager) Preview(ctx context.Context, input Input) (Preview, error) {
	return manager.engine.Preview(ctx, input)
}

// Build creates a preview from already normalized source activity.
func (manager *Manager) Build(input Input, profile Profile, activities []Activity, items []library.Item) Preview {
	return manager.engine.Build(input, profile, activities, items)
}

// Apply commits and consumes one current preview.
func (manager *Manager) Apply(id string) (Summary, error) {
	return manager.engine.Apply(id)
}

// Metrics returns active worker and deferred-run counts.
func (manager *Manager) Metrics() (int, uint64) {
	return len(manager.syncSlots), manager.syncDeferred.Load()
}

func (manager *Manager) validate(input Input) (Input, Profile, error) { //nolint:cyclop // Validation keeps every source credential invariant together.
	input.Source, input.URL = strings.ToLower(strings.TrimSpace(input.Source)), strings.TrimSpace(input.URL)
	input.Token, input.SourceUser, input.ProfileID = strings.TrimSpace(input.Token), strings.TrimSpace(input.SourceUser), strings.TrimSpace(input.ProfileID)
	if input.Source != "plex" && input.Source != "jellyfin" {
		return input, Profile{}, errors.New("source must be plex or jellyfin")
	}
	endpoint, err := url.Parse(input.URL)
	if err != nil || endpoint.Host == "" || endpoint.Scheme != "http" && endpoint.Scheme != "https" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || len(input.URL) > 2048 {
		return input, Profile{}, errors.New("source URL must be an HTTP or HTTPS server URL without credentials, query, or fragment")
	}
	if input.Token == "" || len(input.Token) > 4096 {
		return input, Profile{}, errors.New("source token is required")
	}
	if len(input.SourceUser) > 512 {
		return input, Profile{}, errors.New("source user ID is too long")
	}
	profile, found := manager.config.Profile(input.ProfileID)
	if !found {
		return input, Profile{}, errors.New("destination Viewer Profile was not found")
	}
	input.URL = strings.TrimRight(input.URL, "/")
	return input, profile, nil
}
