package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	sharednavigation "github.com/MikeO7/kinosail/packages/navigation"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

var errManagedSetting = errors.New("setting is externally managed")

type settingsStore struct {
	profiles         *profileStore
	mu               sync.RWMutex
	file, mediaRoot  string
	ffmpeg           string
	value            installationSettings
	config           configuration.Snapshot
	startupConfig    configuration.Snapshot
	subtitleProvider *subtitleProvider
	persist          func(string, any) error
	err              error
}

func (store *settingsStore) changeInstallationSettings(change func(*installationSettings)) error {
	return settingsops.Persist(&store.mu, func() installationSettings { return store.value }, change, store.save, func(value installationSettings) { store.value = value })
}

func newSettingsStore(mediaRoot, dataDir string, stateDB *database.Store, configured ...configuration.Snapshot) *settingsStore {
	store := &settingsStore{mediaRoot: mediaRoot, value: installationSettings{Name: "Kinosail", Libraries: []string{"."}, RequireMFA: true, UpdateChecks: true, SubtitlePickerLimited: true, PickerKeepForced: true, Navigation: sharednavigation.Default()}, persist: statePersistence(stateDB)}
	if len(configured) > 0 {
		store.config = configured[0]
	}
	store.startupConfig = store.config.Clone()
	if dataDir == "" {
		if err := store.ensureServerID(); err != nil {
			store.err = err
			return store
		}
		store.applyConfiguration()
		return store
	}
	store.file = filepath.Join(dataDir, "settings.json")
	if err := store.loadSaved(stateDB); err != nil {
		store.err = err
		return store
	}
	if err := store.ensureServerID(); err != nil {
		store.err = err
		return store
	}
	store.applyConfiguration()
	return store
}

func (store *settingsStore) loadSaved(stateDB *database.Store) error {
	saved := installationSettings{RequireMFA: true, UpdateChecks: true}
	found, err := loadState(stateDB, store.file, &saved)
	if err != nil {
		return err
	}
	if found && saved.Libraries != nil {
		if saved.Name == "" {
			saved.Name = "Kinosail"
		}
		store.value = saved
	}
	if (store.value.PublicSessionInactiveHours != 0 || store.value.PublicSessionAbsoluteHours != 0) && !validSessionTimeouts(store.value.PublicSessionInactiveHours, store.value.PublicSessionAbsoluteHours) {
		return errSessionTimeoutsInvalid
	}
	if sharednavigation.Validate(store.value.Navigation) != nil {
		store.value.Navigation = sharednavigation.Default()
	}
	return nil
}

func (store *settingsStore) applyConfiguration() {
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
		if canonical, ok := subtitlelanguage.NormalizeTag(value); ok {
			settings.SubtitleLanguage = canonical
			settings.SubtitleLanguages = []string{canonical}
		}
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
	settings.SubtitleLanguages = slices.Clone(settings.SubtitleLanguages)
	settings.Navigation = slices.Clone(settings.Navigation)
	if settings.Supporter != nil {
		supporter := cloneSupporterState(*settings.Supporter)
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

func (store *settingsStore) setRequireMFA(required bool) error {
	if err := store.editable("security.require_mfa"); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.RequireMFA = required
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
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
	return cloneSupporterState(*store.value.Supporter)
}

func (store *settingsStore) setSupporter(value supporterState) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	cloned := cloneSupporterState(value)
	settings.Supporter = &cloned
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}
