package configurationcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

func (loader Loader) readYAML(path string) (map[string]string, error) {
	if path == "" {
		return make(map[string]string), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxYAMLSize {
		return nil, errors.New("configuration file must be a regular file no larger than 1 MiB")
	}
	return loader.decodeYAML(file)
}

func (loader Loader) decodeYAML(reader io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxYAMLSize+1))
	if err != nil || len(data) > maxYAMLSize {
		return nil, errors.New("configuration file must be no larger than 1 MiB")
	}
	var root map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("configuration must contain exactly one YAML document")
	}
	version, ok := root["version"].(int)
	if !ok || version != SchemaVersion {
		return nil, errors.New("configuration version must be 1")
	}
	delete(root, "version")
	result := make(map[string]string)
	if err := loader.flatten("", root, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (loader Loader) flatten(prefix string, input map[string]any, output map[string]string) error {
	for key, item := range input {
		name := key
		if prefix != "" {
			name = prefix + "." + key
		}
		if nested, ok := item.(map[string]any); ok {
			if err := loader.flatten(name, nested, output); err != nil {
				return err
			}
			continue
		}
		if _, exists := output[name]; exists {
			return fmt.Errorf("ambiguous YAML setting %q", name)
		}
		raw, err := loader.yamlValue(name, item)
		if err != nil {
			return err
		}
		output[name] = raw
	}
	return nil
}

func (loader Loader) yamlValue(name string, item any) (string, error) { //nolint:cyclop,gocognit // Scores of 19 and 21 remain below the repository ceiling of 22 for one strict scalar decoder.
	invalid := func() (string, error) { return "", fmt.Errorf("invalid %s: wrong YAML value type", name) }
	if loader.retired(name) {
		if value, ok := item.(string); ok {
			return value, nil
		}
		return invalid()
	}
	key := strings.TrimSuffix(name, "_file")
	field, known := loader.find(key)
	if strings.HasSuffix(name, "_file") && (!known || !field.Secret) {
		return "", fmt.Errorf("unknown YAML secret file setting %q", name)
	}
	switch value := item.(type) {
	case string:
		return value, nil
	case bool:
		if !known || field.Kind != Boolean {
			return invalid()
		}
		return strconv.FormatBool(value), nil
	case int:
		if !known || field.Kind != Number {
			return invalid()
		}
		return strconv.Itoa(value), nil
	case []any:
		if !known || field.Kind != List {
			return invalid()
		}
		values := make([]string, len(value))
		for index, item := range value {
			var ok bool
			if values[index], ok = item.(string); !ok {
				return invalid()
			}
		}
		data, _ := json.Marshal(values)
		return string(data), nil
	default:
		return invalid()
	}
}
