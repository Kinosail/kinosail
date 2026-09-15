// Package configurationcore resolves validated configuration values from files and the environment.
package configurationcore

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maxYAMLSize = 1 << 20
	// SchemaVersion is the configuration file schema understood by the shared loader.
	SchemaVersion = 1
)

// Kind identifies the YAML scalar shape accepted by one field.
type Kind string

const (
	// Text accepts only YAML strings.
	Text Kind = "text"
	// Boolean accepts YAML booleans and strings.
	Boolean Kind = "boolean"
	// Number accepts YAML integers and strings.
	Number Kind = "number"
	// List accepts YAML string lists and strings.
	List Kind = "list"
)

// Source records which configuration layer supplied a value.
type Source string

const (
	// Default is the built-in schema value.
	Default Source = "default"
	// GUI is a persisted owner setting.
	GUI Source = "gui"
	// YAML is a configuration file setting.
	YAML Source = "yaml"
	// Environment is an environment variable or secret file setting.
	Environment Source = "environment"
)

// Field describes one app-owned configuration setting.
type Field struct {
	Key, Env, Default string
	Kind              Kind
	Secret            bool
}

// Value is one resolved raw setting and its source.
type Value struct {
	Raw    string
	Source Source
}

// Schema adapts an app's fields and validation to the shared loader.
type Schema struct {
	Fields   []Field
	Retired  func(string) bool
	Validate func(string, string) error
}

// Loader resolves one app's configuration layers.
type Loader struct {
	Schema     Schema
	ReadStored func(string) (map[string]string, map[string]string, error)
	ReadSecret func(string) ([]byte, error)
}

// Load resolves defaults, stored values, YAML, and environment values in precedence order.
func (loader Loader) Load(dataDir, yamlPath string, lookup func(string) (string, bool)) (map[string]Value, error) {
	yamlValues, err := loader.readYAML(yamlPath)
	if err != nil {
		return nil, err
	}
	if configured := yamlValues["paths.data"]; configured != "" {
		dataDir = configured
	}
	if configured, ok := lookup("KINOSAIL_DATA_DIR"); ok && configured != "" {
		dataDir = configured
	}
	regular, secrets, err := loader.ReadStored(dataDir)
	if err != nil {
		return nil, err
	}
	values, err := loader.ResolveStored(regular, secrets)
	if err != nil {
		return nil, err
	}
	if err := loader.applyYAML(values, yamlValues, yamlPath); err != nil {
		return nil, err
	}
	if err := loader.applyEnvironment(values, lookup); err != nil {
		return nil, err
	}
	return values, nil
}

// ResolveStored validates stored maps and overlays them on the schema defaults.
func (loader Loader) ResolveStored(regular, secrets map[string]string) (map[string]Value, error) {
	values := loader.defaults()
	if err := loader.applyStored(values, regular, secrets); err != nil {
		return nil, err
	}
	return values, nil
}

func (loader Loader) defaults() map[string]Value {
	values := make(map[string]Value, len(loader.Schema.Fields))
	for _, field := range loader.Schema.Fields {
		values[field.Key] = Value{Raw: field.Default, Source: Default}
	}
	return values
}

func (loader Loader) applyStored(values map[string]Value, regular, secrets map[string]string) error {
	for _, stored := range []struct {
		values map[string]string
		secret bool
	}{{regular, false}, {secrets, true}} {
		for key, raw := range stored.values {
			if loader.retired(key) {
				continue
			}
			field, ok := loader.find(key)
			if !ok || field.Secret != stored.secret {
				return fmt.Errorf("invalid saved configuration key %q", key)
			}
			if err := loader.Schema.Validate(key, raw); err != nil {
				return err
			}
			values[key] = Value{Raw: raw, Source: GUI}
		}
	}
	return nil
}

func (loader Loader) applyYAML(values map[string]Value, yamlValues map[string]string, yamlPath string) error {
	for key, raw := range yamlValues {
		if loader.retired(key) {
			continue
		}
		resolvedKey, resolvedRaw, err := loader.resolveYAMLSecret(key, raw, yamlPath)
		if err != nil {
			return err
		}
		if _, ok := loader.find(resolvedKey); !ok {
			return fmt.Errorf("unknown YAML setting %q", resolvedKey)
		}
		if err := loader.Schema.Validate(resolvedKey, resolvedRaw); err != nil {
			return err
		}
		values[resolvedKey] = Value{Raw: resolvedRaw, Source: YAML}
	}
	return nil
}

func (loader Loader) resolveYAMLSecret(key, raw, yamlPath string) (string, string, error) {
	canonical, fileSetting := strings.CutSuffix(key, "_file")
	if !fileSetting {
		return key, raw, nil
	}
	field, ok := loader.find(canonical)
	if !ok || !field.Secret {
		return "", "", fmt.Errorf("unknown YAML secret file setting %q", key)
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(filepath.Dir(yamlPath), raw)
	}
	data, err := loader.ReadSecret(raw)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", key, err)
	}
	return canonical, strings.TrimSuffix(string(data), "\n"), nil
}

func (loader Loader) applyEnvironment(values map[string]Value, lookup func(string) (string, bool)) error { //nolint:cyclop,gocognit // Scores of 11 and 19 remain below the repository ceiling of 22 for one precedence pass.
	for _, field := range loader.Schema.Fields {
		direct, directOK := lookup(field.Env)
		file, fileOK := lookup(field.Env + "_FILE")
		directOK, fileOK = directOK && direct != "", fileOK && file != ""
		if directOK && fileOK {
			return fmt.Errorf("%s and %s_FILE cannot both be set", field.Env, field.Env)
		}
		raw := direct
		if fileOK {
			if !field.Secret {
				return fmt.Errorf("%s_FILE is only valid for secrets", field.Env)
			}
			data, err := loader.ReadSecret(file)
			if err != nil {
				return fmt.Errorf("read %s_FILE: %w", field.Env, err)
			}
			raw, directOK = strings.TrimSuffix(string(data), "\n"), true
		}
		if directOK {
			if err := loader.Schema.Validate(field.Key, raw); err != nil {
				return err
			}
			values[field.Key] = Value{Raw: raw, Source: Environment}
		}
	}
	return nil
}

func (loader Loader) find(key string) (Field, bool) {
	for _, field := range loader.Schema.Fields {
		if field.Key == key {
			return field, true
		}
	}
	return Field{}, false
}

func (loader Loader) retired(key string) bool {
	return loader.Schema.Retired != nil && loader.Schema.Retired(key)
}
