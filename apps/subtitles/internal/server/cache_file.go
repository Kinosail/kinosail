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
		_ = os.Remove(temporary) //nolint:gosec // CreateTemp returned this exact path beside the target.
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Link(temporary, target) //nolint:gosec // Linking fails atomically if the owner already has a sidecar at this exact path.
}

func writeDurableFile(target string, data []byte) error {
	return privatefile.Write(target, data)
}
