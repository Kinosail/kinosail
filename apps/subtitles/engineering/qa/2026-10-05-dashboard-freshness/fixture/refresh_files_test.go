package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const r16FixtureSettings = "{\"name\":\"Kinosail\",\"libraries\":[\".\"],\"requireMfa\":true,\"updateChecks\":false,\"homeAssistant\":false,\"jellyfinCompatibility\":false,\"subtitlePickerLimited\":true,\"subtitlePickerKeepForced\":true}\n"

type r16OwnedFiles struct {
	media             *os.Root
	state             *os.Root
	mediaPath         string
	statePath         string
	cachePath         string
	unavailableTool   string
	unavailableDevice string
}

func r16CreateFiles(empty bool) (*r16OwnedFiles, error) {
	parent := os.Getenv("R16_FRESHNESS_PRIVATE_ROOT")
	if err := r16ValidateParent(parent); err != nil {
		return nil, err
	}
	base, err := r16AllocateRoot(parent)
	if err != nil {
		return nil, errors.New("owned R16 retained root unavailable")
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, errors.New("owned R16 retained root could not be opened")
	}
	result, setupErr := r16InitializeFiles(root, base, empty)
	closeErr := root.Close()
	if setupErr != nil || closeErr != nil {
		if result != nil && !result.close() {
			return nil, errors.New("owned R16 filesystem failure did not close its descriptors")
		}
		return nil, errors.New("owned R16 filesystem initialization failed")
	}
	return result, nil
}

func r16InitializeFiles(root *os.Root, base string, empty bool) (*r16OwnedFiles, error) {
	if _, err := root.Lstat("never-installed-r16-device"); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("owned R16 device sentinel was not absent")
	}
	for _, name := range []string{"media", "state"} {
		if err := root.Mkdir(name, 0o700); err != nil {
			return nil, err
		}
	}
	media, err := root.OpenRoot("media")
	if err != nil {
		return nil, err
	}
	state, err := root.OpenRoot("state")
	if err != nil {
		return nil, errors.Join(err, media.Close())
	}
	files := &r16OwnedFiles{
		media: media, state: state,
		mediaPath:         filepath.Join(base, "media"),
		statePath:         filepath.Join(base, "state"),
		cachePath:         filepath.Join(base, "state", "cache"),
		unavailableTool:   filepath.Join(base, "never-installed-r16-tool"),
		unavailableDevice: filepath.Join(base, "never-installed-r16-device"),
	}
	if err = state.Mkdir("cache", 0o700); err != nil {
		return files, err
	}
	if err = state.WriteFile("settings.json", []byte(r16FixtureSettings), 0o600); err != nil {
		return files, err
	}
	if empty {
		return files, nil
	}
	if err = media.WriteFile("R16 Example.mp4", []byte("fictional catalogue only\n"), 0o600); err != nil {
		return files, err
	}
	return files, media.WriteFile("R16 Example.en.srt", []byte(r16InitialText), 0o600)
}

func r16ReadOwned(root *os.Root, name string) ([]byte, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > r16ResponseLimit {
		return nil, false, errors.New("owned R16 file boundary invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, r16ResponseLimit+1))
	closeErr := file.Close()
	if len(data) > r16ResponseLimit {
		return nil, false, errors.Join(readErr, closeErr, errors.New("owned R16 file exceeded its limit"))
	}
	return data, true, errors.Join(readErr, closeErr)
}

func (files *r16OwnedFiles) snapshot() (r16Files, error) {
	current, exists, currentErr := r16ReadOwned(files.media, "R16 Example.en.srt")
	recovery, restorable, recoveryErr := r16ReadOwned(files.media, "R16 Example.en.srt.kinosail.bak")
	return r16Files{
		CurrentExists: exists, Current: current,
		RecoveryExists: restorable, Recovery: recovery,
	}, errors.Join(currentErr, recoveryErr)
}

func (files *r16OwnedFiles) close() bool {
	return errors.Join(files.media.Close(), files.state.Close()) == nil
}

func r16AllocateRoot(parent string) (string, error) {
	if parent == "" {
		return os.MkdirTemp("", "kinosail-r16-public-")
	}
	suffix := make([]byte, 16)
	if _, err := rand.Read(suffix); err != nil {
		return "", errors.New("owned R16 directory identity unavailable")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return "", errors.New("owned R16 parent root unavailable")
	}
	name := "kinosail-r16-public-" + hex.EncodeToString(suffix)
	createErr := root.Mkdir(name, 0o700)
	closeErr := root.Close()
	if errors.Join(createErr, closeErr) != nil {
		return "", errors.New("owned R16 retained child unavailable")
	}
	return filepath.Join(parent, name), nil
}

func r16ValidateParent(parent string) error {
	if parent != "" && (!filepath.IsAbs(parent) || filepath.Clean(parent) != parent || len(parent) > 4096) {
		return errors.New("owned R16 parent root invalid")
	}
	if parent != "" {
		info, err := os.Lstat(filepath.Clean(parent))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("owned R16 parent was not an actual directory")
		}
	}
	return nil
}
