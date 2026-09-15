//go:build !windows

package privatefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var errInjected = errors.New("injected file failure")

func TestReadReportsOpenStatAndReadFailures(t *testing.T) {
	path := privateFixture(t)
	t.Run("open", func(t *testing.T) {
		original := openPrivateFile
		openPrivateFile = func(string) (*os.File, error) { return nil, errInjected }
		t.Cleanup(func() { openPrivateFile = original })
		if _, err := Read(path, 32); !errors.Is(err, errInjected) {
			t.Fatalf("Read() error = %v", err)
		}
	})
	t.Run("stat", func(t *testing.T) {
		original := openPrivateFile
		openPrivateFile = func(path string) (*os.File, error) {
			file, err := os.Open(path)
			if err == nil {
				err = file.Close()
			}
			return file, err
		}
		t.Cleanup(func() { openPrivateFile = original })
		if _, err := Read(path, 32); !errors.Is(err, errInvalid) {
			t.Fatalf("Read() error = %v", err)
		}
	})
	t.Run("read", func(t *testing.T) {
		original := readPrivateData
		readPrivateData = func(io.Reader) ([]byte, error) { return nil, errInjected }
		t.Cleanup(func() { readPrivateData = original })
		if _, err := Read(path, 32); !errors.Is(err, errInvalid) {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestWriteReportsTemporaryAndFileFailures(t *testing.T) {
	t.Run("temporary", func(t *testing.T) {
		original := createPrivateTemp
		createPrivateTemp = func(string, string) (*os.File, error) { return nil, errInjected }
		t.Cleanup(func() { createPrivateTemp = original })
		if err := Write(filepath.Join(t.TempDir(), "state"), nil); !errors.Is(err, errInjected) {
			t.Fatalf("Write() error = %v", err)
		}
	})
	t.Run("file operation", func(t *testing.T) {
		original := createPrivateTemp
		createPrivateTemp = func(directory, pattern string) (*os.File, error) {
			file, err := os.CreateTemp(directory, pattern)
			if err == nil {
				err = file.Close()
			}
			return file, err
		}
		t.Cleanup(func() { createPrivateTemp = original })
		if err := Write(filepath.Join(t.TempDir(), "state"), nil); err == nil {
			t.Fatal("closed temporary file was accepted")
		}
	})
}

func TestCreateReportsSyncAndCloseFailures(t *testing.T) {
	t.Run("sync", func(t *testing.T) {
		original := syncPrivateFile
		syncPrivateFile = func(*os.File) error { return errInjected }
		t.Cleanup(func() { syncPrivateFile = original })
		if err := Create(filepath.Join(t.TempDir(), "marker")); !errors.Is(err, errInjected) {
			t.Fatalf("Create() error = %v", err)
		}
	})
	t.Run("close", func(t *testing.T) {
		original := closePrivateFile
		closePrivateFile = func(file *os.File) error {
			_ = file.Close()
			return errInjected
		}
		t.Cleanup(func() { closePrivateFile = original })
		if err := Create(filepath.Join(t.TempDir(), "marker")); !errors.Is(err, errInjected) {
			t.Fatalf("Create() error = %v", err)
		}
	})
}

func TestSyncDirectoryReportsMissingPath(t *testing.T) {
	if err := syncDirectory(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("syncDirectory() error = %v", err)
	}
}

func TestRemoveReportsEveryDurabilityBoundary(t *testing.T) {
	if err := Remove(""); !errors.Is(err, errInvalid) {
		t.Fatalf("Remove(empty) error = %v", err)
	}
	if err := Remove(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("Remove(missing) error = %v", err)
	}
	path := privateFixture(t)
	if err := Remove(path); err != nil {
		t.Fatalf("Remove(file) error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed file remains: %v", err)
	}
	t.Run("remove", func(t *testing.T) {
		original := removePrivateFile
		removePrivateFile = func(string) error { return errInjected }
		t.Cleanup(func() { removePrivateFile = original })
		if err := Remove("state"); !errors.Is(err, errInjected) {
			t.Fatalf("Remove() error = %v", err)
		}
	})
	t.Run("sync", func(t *testing.T) {
		original := syncRemovedFile
		syncRemovedFile = func(string) error { return errInjected }
		t.Cleanup(func() { syncRemovedFile = original })
		if err := Remove(privateFixture(t)); !errors.Is(err, errInjected) {
			t.Fatalf("Remove() error = %v", err)
		}
	})
}

func privateFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
