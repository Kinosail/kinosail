package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/settingsstate"
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
	if err = seedRestoreStartupSettings(root); err != nil {
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

const restoreStartupSettings = `{"name":"Kinosail","libraries":["."],"requireMfa":true,"updateChecks":false,"homeAssistant":false,"jellyfinCompatibility":false,"subtitlePickerLimited":true,"subtitlePickerKeepForced":true}`

func seedRestoreStartupSettings(root *os.Root) error {
	file, err := root.OpenFile("data/settings.json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("Restore startup settings seed unavailable")
	}
	written, writeErr := file.WriteString(restoreStartupSettings)
	err = errors.Join(writeErr, file.Close())
	if err != nil || written != len(restoreStartupSettings) {
		return errors.New("Restore startup settings seed incomplete")
	}
	return nil
}

// Startup seed isolation is outside the four public Restore controls: those
// controls cannot observe the installation loader's automatic update preference.
func TestRestoreStartupSettingsSeedIsPrivate(t *testing.T) {
	_, root := newRestoreStartupFiles(t)
	for _, name := range []string{".", "media", "data", "cache"} {
		info, err := root.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatal("Restore startup must retain private owned directories")
		}
	}
	assertRestoreStartupSettings(t, readRestoreStartupSettings(t, root))
}

func TestRestoreStartupSeedImportsWithoutChangingMedia(t *testing.T) {
	directory, root := newRestoreStartupFiles(t)
	seed := readRestoreStartupSettings(t, root)
	assertRestoreStartupSettings(t, seed)
	rig := &restoreRig{files: root}
	before := rig.readSidecar(t, false)
	state, err := database.OpenContext(context.Background(), filepath.Join(directory, "data"), false)
	if err != nil || state == nil {
		t.Fatal("Restore startup seed must pass the real private database constructor")
	}
	t.Cleanup(func() {
		if closeErr := state.Close(); closeErr != nil {
			t.Error("Restore startup database handle did not settle")
		}
	})
	var imported json.RawMessage
	found, err := state.LoadJSON("settings.json", &imported)
	if err != nil || !found || !bytes.Equal(seed, imported) {
		t.Fatal("Restore startup must import exactly its newly fabricated settings seed")
	}
	assertRestoreStartupSettings(t, imported)
	if _, err = root.Lstat("data/settings.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Restore startup must recognize real migration consumption of its fabricated seed")
	}
	if !bytes.Equal(before, rig.readSidecar(t, false)) {
		t.Fatal("Restore startup settings import must preserve fictional media bytes")
	}
}

func newRestoreStartupDirectory(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("", "r06-restore-startup-unit-")
	if err != nil {
		t.Fatal("Restore startup owned directory unavailable")
	}
	// Preserve all fabricated inputs for evidence; close handles without removal.
	return directory
}

func newRestoreStartupFiles(t *testing.T) (string, *os.Root) {
	t.Helper()
	directory := newRestoreStartupDirectory(t)
	root, err := prepareRestoreFiles(directory)
	if err != nil {
		t.Fatal("Restore startup owned files unavailable")
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error("Restore startup private root handle did not settle")
		}
	})
	return directory, root
}

func readRestoreStartupSettings(t *testing.T, root *os.Root) []byte {
	t.Helper()
	info, err := root.Lstat("data/settings.json")
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("Restore startup settings seed must be a private owned regular file")
	}
	file, err := root.Open("data/settings.json")
	if err != nil {
		t.Fatal("Restore startup settings seed unavailable")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, settingsstate.MaximumDocumentSize+1))
	if err = errors.Join(readErr, file.Close()); err != nil || len(data) > settingsstate.MaximumDocumentSize {
		t.Fatal("Restore startup settings seed exceeds its valid private boundary")
	}
	return data
}

func assertRestoreStartupSettings(t *testing.T, data []byte) {
	t.Helper()
	var fields map[string]json.RawMessage
	if settingsstate.Validate(data) != nil || json.Unmarshal(data, &fields) != nil {
		t.Fatal("Restore startup seed must satisfy the real strict settings schema")
	}
	for _, name := range []string{"name", "libraries", "requireMfa", "updateChecks", "homeAssistant", "jellyfinCompatibility", "subtitlePickerLimited", "subtitlePickerKeepForced"} {
		if len(fields[name]) == 0 {
			t.Fatal("Restore startup seed must explicitly retain isolation and subtitle defaults")
		}
	}
	var settings struct {
		Name                     string
		Libraries                []string
		RequireMfa               bool
		UpdateChecks             bool
		HomeAssistant            bool
		JellyfinCompatibility    bool
		SubtitlePickerLimited    bool
		SubtitlePickerKeepForced bool
	}
	if json.Unmarshal(data, &settings) != nil {
		t.Fatal("Restore startup seed values unavailable")
	}
	if settings.Name != "Kinosail" || len(settings.Libraries) != 1 || settings.Libraries[0] != "." {
		t.Fatal("Restore startup seed must retain its original library root")
	}
	got := [6]bool{settings.RequireMfa, settings.UpdateChecks, settings.HomeAssistant, settings.JellyfinCompatibility, settings.SubtitlePickerLimited, settings.SubtitlePickerKeepForced}
	if got != [6]bool{true, false, false, false, true, true} {
		t.Fatal("Restore startup seed must retain MFA and subtitle defaults while disabling integrations")
	}
}
