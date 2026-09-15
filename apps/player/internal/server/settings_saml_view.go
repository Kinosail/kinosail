package server

import (
	"net/http"
	"strings"

	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func (store *settingsStore) samlConfiguration(request *http.Request) samlConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	value := func(key string) (string, bool) {
		field := store.config.Public(key)
		return field.Value, field.Configured
	}
	origin := strings.TrimSuffix(scimEndpoint(store.config.String("auth.url"), request), "/scim/v2")
	return federationconfig.NewSAMLConfigurationView(value, origin, configurationControl(store.config, samlConfigurationKeys...))
}
