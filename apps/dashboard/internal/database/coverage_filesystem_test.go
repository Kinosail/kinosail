package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageFilesystemFailuresReleaseStore(t *testing.T) {
	for _, target := range []string{"directory", "database", "wal", "shm", "stat"} {
		t.Run(target, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, Filename)
			failure := errors.New("filesystem rejected operation")
			files := coverageFailingStoreFiles(directory, target, failure)
			store, err := openStore(t.Context(), directory, files)
			if store != nil || !errors.Is(err, failure) {
				t.Fatalf("filesystem failure = %v, %v", store, err)
			}
			if target == "directory" || target == "stat" {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("database created before validation: %v", err)
				}
			}
			assertCoverageFilesystemRecovery(t, directory)
		})
	}
}

func coverageFailingStoreFiles(directory, target string, failure error) storeFiles {
	path := filepath.Join(directory, Filename)
	failedPath := map[string]string{"directory": directory, "database": path, "wal": path + "-wal", "shm": path + "-shm"}[target]
	return storeFiles{
		lstat: func(name string) (os.FileInfo, error) {
			if target == "stat" && name == path {
				return nil, failure
			}
			return os.Lstat(name)
		},
		chmod: func(name string, mode os.FileMode) error {
			if name == failedPath {
				return failure
			}
			return os.Chmod(name, mode)
		},
	}
}

func assertCoverageFilesystemRecovery(t *testing.T, directory string) {
	t.Helper()
	store, err := Open(t.Context(), directory)
	if err != nil {
		t.Fatalf("failed open retained process or database lock: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var board any
	if found, err := store.LoadJSON(t.Context(), "board.json", &board); err != nil || found {
		t.Fatalf("failed open persisted application data: %v, %v", found, err)
	}
}
