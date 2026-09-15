package main

import (
	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/appcli"
)

var configurationCommand = appcli.BindConfigurationCommand(
	configuration.Snapshot.Fields,
	func(field configuration.PublicValue) configuration.Source { return field.Source },
	configuration.Default, configuration.GUI, configuration.YAML, configuration.Environment,
)
