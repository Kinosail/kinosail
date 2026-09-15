package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

func (store *settingsStore) applyDLNAConfiguration() error {
	if store.config.Source("dlna.enabled") == configuration.Default {
		return nil
	}
	settings := store.value
	settings.DLNAToken, _ = settingsops.ConfiguredDLNAToken(true, store.config.Bool("dlna.enabled"), settings.DLNAToken)
	store.value = settings
	return store.save(settings)
}
