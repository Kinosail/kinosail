package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type restoreFile interface {
	Name() string
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type restoreOperations struct {
	mkdirAll   func(string, os.FileMode) error
	createTemp func(string, string) (restoreFile, error)
	lstat      func(string) (os.FileInfo, error)
	rename     func(string, string) error
	remove     func(string) error
}

func defaultRestoreOperations() restoreOperations {
	return restoreOperations{
		mkdirAll: os.MkdirAll,
		createTemp: func(directory, pattern string) (restoreFile, error) {
			return os.CreateTemp(directory, pattern)
		},
		lstat: os.Lstat, rename: os.Rename, remove: os.Remove,
	}
}

func (service *Service) save(staged map[string][]byte, dataDir string, managed []string) error {
	if err := service.restore.mkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	targets := append(append([]string(nil), managed...), service.databaseFilename, service.databaseFilename+"-wal", service.databaseFilename+"-shm")
	if err := service.validateRestoreTargets(dataDir, targets); err != nil {
		return err
	}
	temporary, err := service.stageRestoreFiles(dataDir, staged)
	if err != nil {
		return err
	}
	defer cleanup(temporary)
	originals, err := service.moveRestoreTargets(dataDir, targets)
	if err != nil {
		return errors.Join(err, rollbackRestore(dataDir, nil, originals))
	}
	installed, err := service.installRestoreFiles(dataDir, temporary)
	if err != nil {
		return errors.Join(err, rollbackRestore(dataDir, installed, originals))
	}
	cleanup(originals)
	return nil
}

func (service *Service) stageRestoreFiles(dataDir string, staged map[string][]byte) (map[string]string, error) {
	temporary := make(map[string]string, len(staged))
	for name, data := range staged {
		file, err := service.restore.createTemp(dataDir, ".kinosail-restore-")
		if err != nil {
			cleanup(temporary)
			return nil, err
		}
		temporary[name] = file.Name()
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			cleanup(temporary)
			return nil, err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			cleanup(temporary)
			return nil, err
		}
		if err := file.Close(); err != nil {
			cleanup(temporary)
			return nil, err
		}
	}
	return temporary, nil
}

func (service *Service) moveRestoreTargets(dataDir string, targets []string) (map[string]string, error) {
	originals := make(map[string]string)
	for _, name := range targets {
		path := filepath.Join(dataDir, name)
		if _, err := service.restore.lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return originals, err
		}
		original, err := service.vacantRestorePath(dataDir)
		if err != nil {
			return originals, err
		}
		if err := service.restore.rename(path, original); err != nil {
			return originals, err
		}
		originals[name] = original
	}
	return originals, nil
}

func (service *Service) installRestoreFiles(dataDir string, temporary map[string]string) ([]string, error) {
	installed := make([]string, 0, len(temporary))
	for name, path := range temporary {
		if err := service.restore.rename(path, filepath.Join(dataDir, name)); err != nil {
			return installed, err
		}
		delete(temporary, name)
		installed = append(installed, name)
	}
	return installed, nil
}

func (service *Service) validateRestoreTargets(dataDir string, names []string) error {
	for _, name := range names {
		info, err := service.restore.lstat(filepath.Join(dataDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("invalid restore target %s", name)
		}
	}
	return nil
}

func (service *Service) vacantRestorePath(dataDir string) (string, error) {
	file, err := service.restore.createTemp(dataDir, ".kinosail-restore-original-")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = service.restore.remove(path)
		return "", err
	}
	if err := service.restore.remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func rollbackRestore(dataDir string, installed []string, originals map[string]string) error {
	var failures []error
	for _, name := range installed {
		if err := os.Remove(filepath.Join(dataDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	for name, path := range originals {
		if err := os.Rename(path, filepath.Join(dataDir, name)); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func cleanup(paths map[string]string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
