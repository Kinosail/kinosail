package main

import (
	"errors"
	"io"
	"os"
	"testing"
)

const restoreSidecar = "media/R06 Fictional Restore.en.srt"

func prepareRestoreFiles(directory string) (*os.Root, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, errors.New("fixture directory unavailable")
	}
	if err = initializeRestoreFiles(root); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return root, nil
}

func initializeRestoreFiles(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("fixture directory unavailable")
	}
	entries, readErr := directory.ReadDir(-1)
	if err = errors.Join(readErr, directory.Close()); err != nil || len(entries) != 0 {
		return errors.New("fixture directory must be empty")
	}
	for _, name := range []string{"media", "data", "cache"} {
		if err = root.Mkdir(name, 0o700); err != nil {
			return errors.New("fixture directory unavailable")
		}
	}
	if err = root.WriteFile("media/R06 Fictional Restore.mp4", []byte("R06 Restore-only indexed fixture; no decoded media"), 0o600); err != nil {
		return errors.New("fictional fixture unavailable")
	}
	if err = root.WriteFile(restoreSidecar, []byte(initialRestoreSRT), 0o600); err != nil {
		return errors.New("fictional fixture unavailable")
	}
	return nil
}

func (f *restoreRig) readRestoreSubtitle(backup bool) ([]byte, error) {
	name := restoreSidecar
	if backup {
		name += ".kinosail.bak"
	}
	file, err := f.files.Open(name)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, privateResponseLimit+1))
	err = errors.Join(readErr, file.Close())
	if len(data) > privateResponseLimit {
		err = errors.Join(err, errors.New("fixture subtitle bound exceeded"))
	}
	return data, err
}

func (f *restoreRig) readSidecar(t *testing.T, backup bool) []byte {
	t.Helper()
	data, err := f.readRestoreSubtitle(backup)
	if err != nil {
		t.Fatal("Restore owned sidecar unavailable")
	}
	return data
}
