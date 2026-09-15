package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func (store *settingsStore) changeOIDCConfiguration(issuer, clientID, secret, redirectURL, identityClaim string, reset bool) error {
	settings := federationconfig.NewOIDCConfigurationStore(store.file, &store.mu, store.config.String, store.config.Managed, store.config.Source, configuration.SetOIDC, configuration.DeleteOIDC, store.config.UpdateGUI)
	return settings.Change(federationconfig.OIDCConfig{Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, IdentityClaim: identityClaim}, reset)
}
