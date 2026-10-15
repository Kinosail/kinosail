package server

import (
	"net/http"

	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func (store *settingsStore) samlConfiguration(request *http.Request) samlConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	value := func(key string) (string, bool) {
		field := store.config.Public(key)
		return field.Value, field.Configured
	}
	return federationconfig.NewSAMLConfigurationView(value, federationconfig.RequestOrigin(store.config.String("auth.url"), request), configurationControl(store.config, samlConfigurationKeys...))
}
