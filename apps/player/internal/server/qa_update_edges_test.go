package server

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func qaUpdateSettings(data string) servertest.UpdateSettings {
	settings := newSettingsStore("", data, "", nil)
	return servertest.UpdateSettings{Automatic: settings.updateChecks, SaveAutomatic: settings.setUpdateChecks}
}

func TestManualUpdateCheckValidatesEmptyFormAndRedirects(t *testing.T) {
	servertest.AssertManualUpdateCheckValidatesEmptyFormAndRedirects(t, qaUpdateSettings(""), checkForUpdate)
}

func TestUpdateAdaptersUsePlayerSchemasAndPersistPreference(t *testing.T) {
	servertest.AssertUpdateAdaptersPersistPreference(t, "Player", qaUpdateSettings, updateReleasePolicy(), database.SchemaVersion, configuration.SchemaVersion)
}
