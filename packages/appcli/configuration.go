package appcli

import (
	"errors"
	"fmt"
	"io"
)

// ConfigurationCounts describes the origins reported by config validation.
type ConfigurationCounts struct {
	Defaults    int
	UI          int
	File        int
	Environment int
}

// BindConfigurationCommand adapts an application's typed snapshot to the shared command.
func BindConfigurationCommand[Snapshot, Field any, Source comparable](fields func(Snapshot) []Field, source func(Field) Source, defaults, ui, file, environment Source) func([]string, io.Writer, func(string) (Snapshot, error)) (bool, error) {
	return func(args []string, output io.Writer, load func(string) (Snapshot, error)) (bool, error) {
		return ConfigurationCommand(args, output, func(path string) (ConfigurationCounts, error) {
			configured, err := load(path)
			if err != nil {
				return ConfigurationCounts{}, err
			}
			return CountConfigurationSources(fields(configured), source, defaults, ui, file, environment), nil
		})
	}
}

// ConfigurationCommand validates a configuration file through an app adapter.
func ConfigurationCommand(args []string, output io.Writer, validate func(string) (ConfigurationCounts, error)) (bool, error) {
	if len(args) == 0 || args[0] != "config" {
		return false, nil
	}
	if len(args) < 2 || len(args) > 3 || args[1] != "validate" {
		return true, errors.New("usage: kinosail config validate [FILE]")
	}
	if validate == nil {
		return true, errors.New("configuration validator is not configured")
	}
	path := ""
	if len(args) == 3 {
		path = args[2]
	}
	counts, err := validate(path)
	if err != nil {
		return true, err
	}
	_, err = fmt.Fprintf(output, "Configuration is valid: %d defaults, %d UI, %d file, %d environment.\n", counts.Defaults, counts.UI, counts.File, counts.Environment)
	return true, err
}

// CountConfigurationSources classifies fields without depending on an app's configuration types.
func CountConfigurationSources[Field any, Source comparable](fields []Field, source func(Field) Source, defaults, ui, file, environment Source) ConfigurationCounts {
	counts := make(map[Source]int, 4)
	for _, field := range fields {
		counts[source(field)]++
	}
	return ConfigurationCounts{Defaults: counts[defaults], UI: counts[ui], File: counts[file], Environment: counts[environment]}
}
