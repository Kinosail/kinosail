package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/configurationcore"
)

// SchemaVersion is the configuration file schema supported by this executable.
const SchemaVersion = configurationcore.SchemaVersion

func readJSON(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}
	return parseJSON(data, filepath.Base(path))
}

func parseJSON(data []byte, name string) (map[string]string, error) {
	result := make(map[string]string)
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if result == nil {
		return nil, fmt.Errorf("read %s: must be a JSON object", name)
	}
	return result, nil
}
