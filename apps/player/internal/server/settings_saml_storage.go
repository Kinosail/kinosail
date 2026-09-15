package server

import (
	"github.com/MikeO7/kinosail-player/internal/configuration"
	federationconfig "github.com/MikeO7/kinosail/packages/federation/configuration"
)

func (store *settingsStore) changeSAMLConfiguration(metadataURL, metadataXML, identityAttribute string, reset bool) error {
	configured := federationconfig.SAMLConfigurationStore[configuration.Source]{File: store.file, Lock: &store.mu, Managed: store.config.Managed, Source: store.config.Source, Set: configuration.SetSAML, Delete: configuration.DeleteSAML, Update: store.config.UpdateGUI}
	return configured.Change(federationconfig.SAMLConfig{MetadataURL: metadataURL, MetadataXML: metadataXML, IdentityAttribute: identityAttribute}, reset)
}
