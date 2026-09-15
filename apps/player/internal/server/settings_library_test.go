package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLibrarySettingsRegression(t *testing.T) {
	servertest.RunLibrarySettings(t, settingsRegression())
}
