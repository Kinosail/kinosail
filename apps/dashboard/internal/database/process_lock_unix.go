//go:build !windows

package database

import (
	"errors"
	"os"
	"syscall"
)

func acquireProcessLock(path string) (*os.File, error) {
	return acquireProcessFileLock(path, os.Chmod)
}

func acquireProcessFileLock(path string, chmod func(string, os.FileMode) error) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("dashboard data directory is already in use")
	}
	if err := chmod(path, 0o600); err != nil {
		_ = releaseProcessLock(file)
		return nil, err
	}
	return file, nil
}

func releaseProcessLock(file *os.File) error {
	if file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return errors.Join(unlockErr, file.Close())
}
