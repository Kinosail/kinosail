package server

import (
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func writeAtomicFile(target string, data []byte) error {
	return privatefile.WriteCache(target, data)
}

func writeAtomicFileExclusive(target string, data []byte) error {
	directory := filepath.Dir(target)
	file, err := os.CreateTemp(directory, "."+filepath.Base(target)+"-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(temporary)
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Link(temporary, target); err != nil {
		return err
	}
	syncDirectory(directory)
	return nil
}

func syncDirectory(path string) {
	if directory, err := os.Open(path); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
}

func writeDurableFile(target string, data []byte) error {
	return privatefile.Write(target, data)
}
