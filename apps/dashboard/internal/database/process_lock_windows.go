//go:build windows

package database

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func acquireProcessLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped); err != nil {
		_ = file.Close()
		return nil, errors.New("dashboard data directory is already in use")
	}
	return file, nil
}

func releaseProcessLock(file *os.File) error {
	if file == nil {
		return nil
	}
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	return errors.Join(unlockErr, file.Close())
}
