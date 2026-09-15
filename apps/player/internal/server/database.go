package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func configuredDatabase(databases []*database.Store) *database.Store {
	if len(databases) == 0 {
		return nil
	}
	return databases[0]
}

func loadState(database *database.Store, path string, target any) (bool, error) {
	if database != nil {
		return database.LoadJSON(filepath.Base(path), target)
	}
	data, err := os.ReadFile(path) //nolint:gosec // Callers provide fixed installation-owned state paths.
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, target)
}

func statePersistence(database *database.Store) func(string, any) error {
	if database == nil {
		return saveJSON
	}
	return func(path string, value any) error {
		return database.SaveJSON(filepath.Base(path), value)
	}
}
