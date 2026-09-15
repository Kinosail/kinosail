package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
)

func configuredDatabase(databases []*database.Store) *database.Store {
	if len(databases) == 0 {
		return nil
	}
	return databases[0]
}

func loadState(stateDB *database.Store, path string, target any) (bool, error) { //nolint:cyclop // File, size, JSON, and database recovery checks are one boundary.
	if stateDB != nil {
		return stateDB.LoadJSON(filepath.Base(path), target)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()
	name := filepath.Base(path)
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > database.DocumentSizeLimit {
		return false, fmt.Errorf("invalid persisted state %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, database.DocumentSizeLimit+1))
	if err != nil || len(data) > database.DocumentSizeLimit {
		return false, fmt.Errorf("invalid persisted state %s", name)
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
