package markers

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

func loadState(database *documentdb.Store, path string, target any) (bool, error) {
	if database != nil {
		data, found, err := database.Load(filepath.Base(path))
		if err != nil || !found {
			return found, err
		}
		return true, httpguard.DecodeJSON(bytes.NewReader(data), documentdb.DocumentSizeLimit, target, true)
	}
	file, err := os.Open(path) //nolint:gosec // The path is fixed below the installation data directory.
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > documentdb.DocumentSizeLimit {
		return false, errors.New("invalid marker state file")
	}
	return true, httpguard.DecodeJSON(file, documentdb.DocumentSizeLimit, target, true)
}

func validateRecords(records map[string]Record) error { //nolint:cyclop // Persisted record identity, marker, and suppression bounds form one document contract.
	if len(records) > 100000 {
		return errors.New("too many persisted marker records")
	}
	for id, record := range records {
		if !validRecordID(id) || record.Revision == "" || len(record.Revision) > 128 || hasControl(record.Revision) || record.DetectorVersion < 0 || record.DetectorVersion > DetectorVersion || len(record.Markers) > 256 || len(record.Suppressed) > len(markerTypes) {
			return errors.New("persisted marker record is invalid")
		}
		for _, marker := range record.Markers {
			if !validPersistedMarker(marker) {
				return errors.New("persisted marker record is invalid")
			}
		}
		if suppressed, err := NormalizeAutoSkip(record.Suppressed); err != nil || len(suppressed) != len(record.Suppressed) {
			return errors.New("persisted marker record is invalid")
		}
	}
	return nil
}

func validRecordID(value string) bool {
	return value != "" && len(value) <= 128 && !strings.ContainsAny(value, `/\`) && !hasControl(value)
}

func validPersistedMarker(marker Marker) bool {
	return slices.Contains(markerTypes, marker.Type) && marker.Label != "" && len(marker.Label) <= 64 && !hasControl(marker.Label) &&
		slices.Contains([]string{"chapter", "fingerprint", "manual", "recurrence", "visual"}, marker.Source) && finite(marker.Start) && finite(marker.End) && marker.Start >= 0 && marker.End > marker.Start
}

func hasControl(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }

func statePersistence(database *documentdb.Store) func(string, any) error {
	if database == nil {
		return saveJSON
	}
	return func(path string, value any) error { return database.SaveJSON(filepath.Base(path), value) }
}

type markerStateFile interface {
	Name() string
	Write([]byte) (int, error)
	Chmod(os.FileMode) error
	Close() error
}

func saveJSON(path string, value any) error {
	return saveJSONWithFile(path, value, createMarkerStateFile)
}

func createMarkerStateFile(directory string) (markerStateFile, error) {
	return os.CreateTemp(directory, ".markers-*")
}

func saveJSONWithFile(path string, value any, create func(string) (markerStateFile, error)) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := create(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}
