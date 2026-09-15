package main

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestConfigurationCommandValidatesAndSummarizesSources(t *testing.T) {
	commandtest.ConfigurationCommandSources(t, configuration.Set, configuration.Load, configurationCommand)
}
