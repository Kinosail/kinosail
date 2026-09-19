package main

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/commandtest"
)

func TestCommandConfigurationEdges(t *testing.T) {
	commandtest.ConfigurationEdges(t, configuration.Load, configuredAuthURL, "38127")
}

func TestLoadConfigurationFindsDefaultFile(t *testing.T) {
	commandtest.DefaultConfigurationFile(t, loadConfigurationPath)
}
