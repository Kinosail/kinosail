package server

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func qaUpdateSettings(data string) servertest.UpdateSettings {
	settings := newSettingsStore("", data, "", nil)
	return servertest.UpdateSettings{Automatic: settings.updateChecks, SaveAutomatic: settings.setUpdateChecks}
}

func TestManualUpdateCheckValidatesEmptyFormAndRedirects(t *testing.T) {
	servertest.AssertManualUpdateCheckValidatesEmptyFormAndRedirects(t, qaUpdateSettings(""), checkForUpdate)
}

func TestUpdateAdaptersUseSubtitlesSchemasAndPersistPreference(t *testing.T) {
	servertest.AssertUpdateAdaptersPersistPreference(t, "Subtitles", qaUpdateSettings, updateReleasePolicy(), database.SchemaVersion, configuration.SchemaVersion)
}
