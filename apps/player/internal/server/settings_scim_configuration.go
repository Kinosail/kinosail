package server

import (
	"net/http"

	scimapp "github.com/MikeO7/kinosail/packages/scim/app"
)

func (store *settingsStore) scimConfiguration(request *http.Request) scimConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	token := store.config.Public(scimapp.TokenKey)
	state := scimapp.ConfigurationState{Origin: store.config.String("auth.url"), Expiration: store.config.String(scimapp.ExpirationKey), Configured: token.Configured}
	return scimapp.NewConfigurationView(state, configurationControl(store.config, scimConfigurationKeys...), request)
}
