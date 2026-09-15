package server

import federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"

const (
	oidcConfigurationKey  = federationconfig.OIDCConfigurationKey
	oidcConfigurationHTML = federationconfig.OIDCConfigurationHTML
)

var oidcConfigurationKeys = federationconfig.OIDCConfigurationKeys()

type oidcConfigurationView = federationconfig.OIDCConfigurationView[settingControl]

func (store *settingsStore) oidcConfiguration() oidcConfigurationView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	value := func(key string) (string, bool) {
		field := store.config.Public(key)
		return field.Value, field.Configured
	}
	return federationconfig.NewOIDCConfigurationView(value, configurationControl(store.config, oidcConfigurationKeys...))
}
