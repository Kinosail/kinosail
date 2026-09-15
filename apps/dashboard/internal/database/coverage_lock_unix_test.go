//go:build !windows

package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageProcessLockErrors(t *testing.T) {
	if lock, err := acquireProcessLock(t.TempDir()); err == nil || lock != nil {
		t.Fatal("directory lock accepted")
	}
	if err := releaseProcessLock(nil); err != nil {
		t.Fatalf("nil unlock = %v", err)
	}
}

func TestCoverageProcessLockPermissionFailureReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "process.lock")
	failure := errors.New("lock permission failure")
	lock, err := acquireProcessFileLock(path, func(string, os.FileMode) error { return failure })
	if lock != nil || !errors.Is(err, failure) {
		t.Fatalf("lock permission failure = %v, %v", lock, err)
	}
	lock, err = acquireProcessLock(path)
	if err != nil {
		t.Fatalf("failed permissions retained lock: %v", err)
	}
	if err := releaseProcessLock(lock); err != nil {
		t.Fatal(err)
	}
}
