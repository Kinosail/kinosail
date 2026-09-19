package main

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestCommandConfigurationEdges(t *testing.T) {
	commandtest.ConfigurationEdges(t, configuration.Load, configuredAuthURL, "38128")
}

func TestLoadConfigurationFindsDefaultFile(t *testing.T) {
	commandtest.DefaultConfigurationFile(t, loadConfigurationPath)
}
