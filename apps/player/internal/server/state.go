package server

import (
	"encoding/json"
)

func saveJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, data)
}
