package mcpgateway

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestApplicationStateUsesInstallationPath(t *testing.T) {
	dir := t.TempDir()
	value := &struct{ Name string }{Name: "saved connection"}
	readErr, writeErr := errors.New("read failed"), errors.New("write failed")
	reads, writes := 0, 0
	store := ApplicationState(dir, func(path string, target any) (bool, error) {
		reads++
		if path != filepath.Join(dir, StateFilename) || target != value {
			t.Fatalf("unexpected load: %q, %v", path, target)
		}
		return true, readErr
	}, func(path string, saved any) error {
		writes++
		if path != filepath.Join(dir, StateFilename) || saved != value {
			t.Fatalf("unexpected save: %q, %v", path, saved)
		}
		return writeErr
	})
	if found, err := store.Load(value); !found || !errors.Is(err, readErr) {
		t.Fatalf("load result = %v, %v", found, err)
	}
	if err := store.Save(value); !errors.Is(err, writeErr) {
		t.Fatalf("save error = %v", err)
	}
	if reads != 1 || writes != 1 {
		t.Fatalf("persistence calls = %d reads, %d writes", reads, writes)
	}
}

func TestApplicationStateWithoutDirectorySkipsLoad(t *testing.T) {
	writes := 0
	store := ApplicationState("", func(string, any) (bool, error) {
		t.Fatal("empty directory must not load a working-directory file")
		return true, nil
	}, func(path string, value any) error {
		writes++
		if path != StateFilename || value != "connection" {
			t.Fatalf("unexpected save: %q, %v", path, value)
		}
		return nil
	})
	if found, err := store.Load(nil); found || err != nil || writes != 0 {
		t.Fatalf("empty-directory load = %v, %v; writes = %d", found, err, writes)
	}
	if err := store.Save("connection"); err != nil || writes != 1 {
		t.Fatalf("save = %v; writes = %d", err, writes)
	}
}

func TestApplicationStatePreservesMissingState(t *testing.T) {
	store := ApplicationState(t.TempDir(), func(string, any) (bool, error) {
		return false, nil
	}, func(string, any) error {
		t.Fatal("reading missing state must not save")
		return nil
	})
	if found, err := store.Load(new(string)); found || err != nil {
		t.Fatalf("missing-state load = %v, %v", found, err)
	}
}
