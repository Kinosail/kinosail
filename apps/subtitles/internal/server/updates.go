package server

import (
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

type (
	updateStatus  = updatecontrol.Status
	updateChecker = updatecontrol.Checker
)

func updateReleasePolicy() updatecontrol.Policy {
	return updatecontrol.SubtitlesPolicy(database.SchemaVersion, configuration.SchemaVersion)
}

func (store *settingsStore) updateChecks() bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.UpdateChecks
}

func (store *settingsStore) setUpdateChecks(enabled bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.UpdateChecks = enabled
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func newUpdateChecker(settings *settingsStore, manager *updatecontrol.Store) *updateChecker {
	checker, err := updatecontrol.NewGitHubChecker(hardenedHTTPClient(10*time.Second), updateReleasePolicy(), updatecontrol.CheckerConfig{
		Manager: manager, CurrentVersion: ApplicationVersion,
		Automatic: settings.updateChecks, SaveAutomatic: settings.setUpdateChecks,
	})
	if err != nil {
		panic(err)
	}
	return checker
}
