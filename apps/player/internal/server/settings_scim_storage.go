package server

import (
	"github.com/MikeO7/kinosail-player/internal/configuration"
	scimapp "github.com/MikeO7/kinosail/packages/scim/app"
)

func (store *settingsStore) changeSCIMConfiguration(token, expiresAt string, reset bool) error {
	settings := scimapp.ConfigurationStore[configuration.Source]{
		File: store.file, Lock: &store.mu, Current: store.config.String,
		Managed: store.config.Managed, Source: store.config.Source,
		Set: configuration.SetSCIM, Delete: configuration.DeleteSCIM, Update: store.config.UpdateGUI,
	}
	return settings.Change(token, expiresAt, reset)
}
