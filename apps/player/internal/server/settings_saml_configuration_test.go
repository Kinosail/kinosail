package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var samlConfigurationContracts = servertest.SAMLConfiguration[configuration.Snapshot]{
	Load: configuration.Load, SignIn: signInTestProfile, WebCall: requestWithCookie, IDPMetadata: samlIDPMetadata,
	New: func(directory string, configured configuration.Snapshot) http.Handler {
		return server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	},
}

func TestOwnerCanConfigureSAMLWithProviderMetadataURL(t *testing.T) {
	samlConfigurationContracts.OwnerCanConfigureSAMLWithProviderMetadataURL(t)
}

func TestSettingsShowDownloadedSAMLMetadataAsConfigured(t *testing.T) {
	samlConfigurationContracts.SettingsShowDownloadedSAMLMetadataAsConfigured(t)
}
