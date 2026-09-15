package server

import (
	"encoding/json"
)

func saveJSON(path string, value any) error {
	return saveJSONFile(path, value, false)
}

func saveDurableJSON(path string, value any) error {
	return saveJSONFile(path, value, true)
}

func saveJSONFile(path string, value any, durable bool) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if durable {
		return writeDurableFile(path, data)
	}
	return writeAtomicFile(path, data)
}
