package configuration_test

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/configurationtest"
)

func TestSharedConfigurationContract(t *testing.T) {
	configurationtest.Run(t, configuration.Load, configuration.Set, configuration.Delete,
		configuration.TrustedOrigin, (*configuration.Snapshot).UpdateGUI,
		configuration.Default, configuration.GUI, configuration.YAML, configuration.Environment, "38128")
}
