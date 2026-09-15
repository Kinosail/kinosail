// Package privatefile owns bounded reads and durable replacement of owner-only files.
package privatefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

var errInvalid = errors.New("private file is invalid")

var (
	openPrivateFile   = os.Open
	readPrivateData   = io.ReadAll
	createPrivateTemp = os.CreateTemp
	syncPrivateFile   = func(file *os.File) error { return file.Sync() }
	closePrivateFile  = func(file *os.File) error { return file.Close() }
	openPrivateMarker = os.OpenFile
	removePrivateFile = os.Remove
	syncRemovedFile   = syncDirectory
)

// Read returns one bounded regular file that only its owner can access.
func Read(path string, maximum int64) ([]byte, error) {
	if path == "" || maximum < 1 {
		return nil, errInvalid
	}
	linked, err := inspect(path, maximum)
	if err != nil {
		return nil, err
	}
	file, err := openPrivateFile(path) //nolint:gosec // Callers pass installation-owned paths.
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(linked, opened) || !valid(opened, maximum) {
		return nil, errInvalid
	}
	data, err := readPrivateData(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, errInvalid
	}
	return data, nil
}

func inspect(path string, maximum int64) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !valid(info, maximum) || info.Mode()&os.ModeSymlink != 0 {
		return nil, errInvalid
	}
	return info, nil
}

func valid(info os.FileInfo, maximum int64) bool {
	return info.Mode().IsRegular() && securePermissions(info) && info.Size() <= maximum
}

// Write atomically replaces one owner-only file and syncs its directory entry.
func Write(path string, data []byte) error {
	return write(path, data, true)
}

// WriteCache atomically replaces owner-only cache data without a durability sync.
func WriteCache(path string, data []byte) error {
	return write(path, data, false)
}

// Remove durably removes one owner-only file if it exists.
func Remove(path string) error {
	if path == "" {
		return errInvalid
	}
	err := removePrivateFile(path) //nolint:gosec // Callers pass installation-owned paths.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncRemovedFile(filepath.Dir(path))
}

func write(path string, data []byte, durable bool) error { //nolint:cyclop // Atomic durable writes require each cleanup boundary to remain explicit.
	if path == "" {
		return errInvalid
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil { //nolint:gosec // Callers pass installation-owned paths.
		return err
	}
	file, err := createPrivateTemp(directory, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(temporary) //nolint:gosec // CreateTemp returned this exact path.
	}()
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil && durable {
		err = syncPrivateFile(file)
	}
	if closeErr := closePrivateFile(file); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil { //nolint:gosec // Callers pass installation-owned paths.
		return err
	}
	if durable {
		return syncDirectory(directory)
	}
	return nil
}

// Create commits one empty owner-only marker without replacing an existing file.
func Create(path string) error {
	if path == "" {
		return errInvalid
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil { //nolint:gosec // Callers pass installation-owned paths.
		return err
	}
	file, err := openPrivateMarker(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // Callers pass installation-owned paths.
	if err != nil {
		return err
	}
	if err = syncPrivateFile(file); err != nil {
		_ = closePrivateFile(file)
		return err
	}
	if err = closePrivateFile(file); err != nil {
		return err
	}
	return syncDirectory(directory)
}
