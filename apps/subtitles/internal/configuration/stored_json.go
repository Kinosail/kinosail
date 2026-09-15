package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

const maxStoredJSONSize = 1 << 20

func readJSON(path string) (map[string]string, error) {
	data, err := privatefile.Read(path, maxStoredJSONSize)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, storedJSONError(filepath.Base(path))
	}
	return parseJSON(data, filepath.Base(path))
}

func parseJSON(data []byte, name string) (map[string]string, error) { //nolint:cyclop // The token loop rejects each ambiguous JSON shape at one boundary.
	if len(data) > maxStoredJSONSize || !utf8.Valid(data) {
		return nil, storedJSONError(name)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, storedJSONError(name)
	}
	result := make(map[string]string)
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		_, duplicate := result[key]
		if err != nil || !ok || duplicate || len(result) >= len(specs) {
			return nil, storedJSONError(name)
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, storedJSONError(name)
		}
		result[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, storedJSONError(name)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, storedJSONError(name)
	}
	return result, nil
}

func storedJSONError(name string) error {
	return fmt.Errorf("read %s: must be one JSON object no larger than 1 MiB with unique keys and string values", name)
}
