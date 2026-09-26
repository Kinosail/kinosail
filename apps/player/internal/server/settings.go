package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/database"
	sharednavigation "github.com/MikeO7/kinosail/packages/navigation"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var errManagedSetting = errors.New("setting is externally managed")

type (
	navigationLink       = sharednavigation.Link
	navigationPreference = sharednavigation.Preference
)

type installationSettings struct {
	Name                  string   `json:"name"`
	Libraries             []string `json:"libraries"`
	RequireMFA            bool     `json:"requireMfa"`
	SessionInactiveHours  float64  `json:"sessionInactiveHours,omitempty"`
	SessionAbsoluteHours  float64  `json:"sessionAbsoluteHours,omitempty"`
	JellyfinCompatibility bool     `json:"jellyfinCompatibility"`
	HomeAssistant         bool     `json:"homeAssistant,omitempty"`
	JellyfinID            string   `json:"jellyfinId,omitempty"`
	settingsops.Playback
	transcodehardware.Selection
	SubtitleLanguage  string          `json:"subtitleLanguage,omitempty"`
	ScanFrequency     string          `json:"scanFrequency,omitempty"`
	DLNAToken         string          `json:"dlnaToken,omitempty"`
	Navigation        []string        `json:"navigation"`
	OnboardingPending bool            `json:"onboardingPending,omitempty"`
	UpdateChecks      bool            `json:"updateChecks"`
	SupporterDisplay  string          `json:"supporterDisplay,omitempty"`
	Supporter         *supporterState `json:"supporter,omitempty"`
}

type settingsStore struct {
	mu                sync.RWMutex
	file              string
	mediaRoot         string
	dlnaURL           string
	ffmpeg            string
	value             installationSettings
	config            configuration.Snapshot
	metadata          *metadataStore
	metadataChanged   func()
	tmdbCheck         func(context.Context, string, string) error
	hardware          hardwareCapabilities
	transcoderCheck   transcodepolicy.CheckResult
	trustedHTTPSCheck func(context.Context, trustedhttps.Config) error
	persist           func(string, any) error
	err               error
}

func (store *settingsStore) transcoderState() transcodehardware.SelectionState {
	return transcodehardware.SelectionState{Lock: &store.mu, Value: &store.value.Selection, Save: func() error { return store.save(store.value) }, Check: &store.transcoderCheck}
}

func (store *settingsStore) changeInstallationSettings(change func(*installationSettings)) error {
	return settingsops.Persist(&store.mu, func() installationSettings { return store.value }, change, store.save, func(value installationSettings) { store.value = value })
}

func newSettingsStore(mediaRoot, dataDir, dlnaURL string, stateDB *database.Store, configured ...configuration.Snapshot) *settingsStore {
	store := &settingsStore{mediaRoot: mediaRoot, dlnaURL: dlnaURL, value: installationSettings{Name: "Kinosail", Libraries: []string{"."}, RequireMFA: true, UpdateChecks: true, Navigation: sharednavigation.Default()}, trustedHTTPSCheck: func(ctx context.Context, config trustedhttps.Config) error { return trustedhttps.Check(ctx, config) }, persist: statePersistence(stateDB)}
	store.tmdbCheck = checkTMDBToken
	if len(configured) > 0 {
		store.config = configured[0]
	}
	if dataDir == "" {
		if err := store.ensureJellyfinID(); err != nil {
			store.err = err
			return store
		}
		store.applyConfiguration()
		store.err = store.applyDLNAConfiguration()
		return store
	}
	store.file = filepath.Join(dataDir, "settings.json")
	saved := installationSettings{RequireMFA: true, UpdateChecks: true}
	found, err := loadState(stateDB, store.file, &saved)
	if err != nil {
		store.err = err
		return store
	}
	if found && saved.Libraries != nil {
		if saved.Name == "" {
			saved.Name = "Kinosail"
		}
		store.value = saved
	}
	if sharednavigation.Validate(store.value.Navigation) != nil {
		store.value.Navigation = sharednavigation.Default()
	}
	if err := store.ensureJellyfinID(); err != nil {
		store.err = err
		return store
	}
	store.applyConfiguration()
	store.err = store.applyDLNAConfiguration()
	return store
}

func (store *settingsStore) applyConfiguration() { //nolint:cyclop,gocognit // Each supported live setting is mapped explicitly from typed configuration.
	settings := store.value
	configured := func(key string) bool {
		source := store.config.Source(key)
		return source != "" && source != configuration.Default
	}
	if value := store.config.String("server.name"); value != "" {
		settings.Name = value
	}
	if configured("security.require_mfa") {
		settings.RequireMFA = store.config.Bool("security.require_mfa")
	}
	if configured("libraries") {
		settings.Libraries = store.config.Strings("libraries")
	}
	if value := store.config.String("playback.mode"); value != "" {
		settings.PlaybackMode = value
	}
	if configured("playback.autoplay") {
		settings.Autoplay = store.config.Bool("playback.autoplay")
	}
	if value := store.config.String("playback.subtitles"); value != "" {
		settings.Subtitles = value
	}
	if configured("playback.auto_skip") {
		settings.AutoSkip = store.config.Strings("playback.auto_skip")
	}
	if value := store.config.String("transcoding.quality"); value != "" {
		settings.Transcoder = value
	}
	if value := store.config.String("transcoding.codec"); value != "" {
		settings.Codec = value
	}
	if value := store.config.String("transcoding.accelerator"); value != "" {
		settings.Accelerator = value
	}
	if configured("transcoding.tone_map") {
		settings.ToneMap = store.config.Bool("transcoding.tone_map")
	}
	if value := store.config.String("subtitles.language"); value != "" {
		settings.SubtitleLanguage = value
	}
	if configured("integrations.jellyfin.enabled") {
		settings.JellyfinCompatibility = store.config.Bool("integrations.jellyfin.enabled")
	}
	if configured("integrations.home_assistant.enabled") {
		settings.HomeAssistant = store.config.Bool("integrations.home_assistant.enabled")
	}
	if value := store.config.String("scanning.frequency"); value != "" {
		settings.ScanFrequency = value
	}
	store.value = settings
}

func (store *settingsStore) editable(keys ...string) error {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.editableLocked(keys...)
}

func (store *settingsStore) editableLocked(keys ...string) error {
	for _, key := range keys {
		if store.config.Managed(key) {
			return fmt.Errorf("%w: %s is managed by %s", errManagedSetting, key, store.config.Source(key))
		}
	}
	return nil
}

func (store *settingsStore) configuration() configuration.Snapshot {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.config.Clone()
}

func (store *settingsStore) setName(name string) error {
	if err := store.editable("server.name"); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return errors.New("server name must contain 1 to 64 characters")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.Name = name
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) serverName() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.Name
}

func (store *settingsStore) snapshot() installationSettings {
	store.mu.RLock()
	defer store.mu.RUnlock()
	settings := store.value
	settings.Libraries = slices.Clone(settings.Libraries)
	settings.AutoSkip = slices.Clone(settings.AutoSkip)
	settings.Navigation = slices.Clone(settings.Navigation)
	if settings.Supporter != nil {
		supporter := *settings.Supporter
		settings.Supporter = &supporter
	}
	return settings
}

func (store *settingsStore) onboardingPending() bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.OnboardingPending
}

func (store *settingsStore) setOnboardingPending(pending bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.OnboardingPending = pending
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) requireMFA() bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.RequireMFA
}

func (store *settingsStore) jellyfinID() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.JellyfinID
}

func (store *settingsStore) save(settings installationSettings) error {
	if store.file == "" {
		return nil
	}
	return store.persist(store.file, settings)
}

func (store *settingsStore) supporter() supporterState {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.value.Supporter == nil {
		return supporterState{}
	}
	return *store.value.Supporter
}

func (store *settingsStore) setSupporter(value supporterState) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.Supporter = &value
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}
