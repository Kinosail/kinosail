package markers

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

type failingMarkerStateFile struct {
	name      string
	writeErr  error
	chmodErr  error
	closeErr  error
	wrote     bool
	chmodded  bool
	closeCall bool
}

func (file *failingMarkerStateFile) Name() string { return file.name }

func (file *failingMarkerStateFile) Write(data []byte) (int, error) {
	file.wrote = true
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return len(data), nil
}

func (file *failingMarkerStateFile) Chmod(os.FileMode) error {
	file.chmodded = true
	return file.chmodErr
}

func (file *failingMarkerStateFile) Close() error {
	file.closeCall = true
	return file.closeErr
}

func requireMissingMarkerState(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatal("marker state exists after failure")
	}
}

func TestMarkerStateValidFile(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	validPath := filepath.Join(directory, "valid.json")
	if err := os.WriteFile(validPath, []byte(`{"value":"ok"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if found, err := loadState(nil, validPath, &value); err != nil || !found || value["value"] != "ok" {
		t.Fatalf("valid state = %#v, %v, %v", value, found, err)
	}
}

func TestMarkerStateFileShapeBoundaries(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	empty := filepath.Join(directory, "empty.json")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(directory, "oversized.json")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(documentdb.DocumentSizeLimit + 1); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	for name, path := range map[string]string{
		"directory": directory,
		"empty":     empty,
		"oversized": oversized,
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]string
			if found, err := loadState(nil, path, &value); err == nil || found {
				t.Fatalf("invalid state = %v, %v", found, err)
			}
		})
	}
}

func TestMarkerStateFileOpenAndDecodeFailures(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	var value map[string]string
	parentFile := filepath.Join(directory, "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if found, err := loadState(nil, filepath.Join(parentFile, "state.json"), &value); err == nil || found {
		t.Fatalf("open failure = %v, %v", found, err)
	}
	malformed := filepath.Join(directory, "malformed.json")
	if err := os.WriteFile(malformed, []byte(`{"value":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if found, err := loadState(nil, malformed, &value); err == nil || !found {
		t.Fatalf("malformed state = %v, %v", found, err)
	}
}

func TestMarkerDatabaseStateAndPersistence(t *testing.T) { //nolint:cyclop // One database lifecycle verifies every persisted marker field.
	t.Parallel()
	directory := t.TempDir()
	store, err := documentdb.Open(directory, false, documentdb.Config{Documents: []string{"state.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var value map[string]string
	if found, loadErr := loadState(store, filepath.Join(directory, "state.json"), &value); loadErr != nil || found {
		t.Fatalf("missing database state = %v, %v", found, loadErr)
	}
	persist := statePersistence(store)
	if err = persist(filepath.Join("ignored", "state.json"), map[string]string{"value": "ok"}); err != nil {
		t.Fatal(err)
	}
	if found, loadErr := loadState(store, filepath.Join(directory, "state.json"), &value); loadErr != nil || !found || value["value"] != "ok" {
		t.Fatalf("database state = %#v, %v, %v", value, found, loadErr)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if found, loadErr := loadState(store, filepath.Join(directory, "state.json"), &value); loadErr == nil || found {
		t.Fatalf("closed database state = %v, %v", found, loadErr)
	}
}

func TestPersistedMarkerValidationBoundaries(t *testing.T) { //nolint:gocognit // One table covers the persisted document contract.
	t.Parallel()
	marker := Marker{Type: "intro", Label: "Intro", Start: 1, End: 2, Source: "manual"}
	valid := Record{Revision: "revision", DetectorVersion: DetectorVersion, Markers: []Marker{marker}, Suppressed: []string{"credits"}}
	if err := validateRecords(map[string]Record{"item": valid}); err != nil {
		t.Fatal(err)
	}
	tooMany := make(map[string]Record, 100001)
	for index := 0; index < 100001; index++ {
		tooMany[strconv.Itoa(index)] = valid
	}
	if err := validateRecords(tooMany); err == nil || err.Error() != "too many persisted marker records" {
		t.Fatalf("record limit error = %v", err)
	}
	tests := map[string]map[string]Record{
		"invalid record":      {"": valid},
		"invalid marker":      {"item": {Revision: "revision", Markers: []Marker{{Type: "intro"}}}},
		"invalid suppression": {"item": {Revision: "revision", Suppressed: []string{"intro", "intro"}}},
	}
	for name, records := range tests {
		t.Run(name, func(t *testing.T) {
			if err := validateRecords(records); err == nil || err.Error() != "persisted marker record is invalid" {
				t.Fatalf("validation error = %v", err)
			}
		})
	}
}

func TestSaveMarkerStateSuccessAndDeterministicFailures(t *testing.T) { //nolint:cyclop // One filesystem matrix proves all atomic-write failure boundaries.
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "nested", "state.json")
	if err := statePersistence(nil)(path, map[string]string{"value": "ok"}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != `{"value":"ok"}` {
		t.Fatalf("saved state = %q, %v", data, err)
	}
	marshalPath := filepath.Join(directory, "marshal.json")
	if err := saveJSON(marshalPath, make(chan int)); err == nil {
		t.Fatal("unsupported JSON value was saved")
	}
	requireMissingMarkerState(t, marshalPath)
	parentFile := filepath.Join(directory, "parent")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateBelowFile := filepath.Join(parentFile, "state.json")
	if err := saveJSON(stateBelowFile, map[string]string{}); err == nil {
		t.Fatal("state was saved below a file")
	}
	requireMissingMarkerState(t, stateBelowFile)
	blocked := filepath.Join(directory, "blocked")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	blockedState := filepath.Join(blocked, "state.json")
	if err := saveJSON(blockedState, map[string]string{}); err == nil {
		t.Fatal("state was saved in an unwritable directory")
	}
	requireMissingMarkerState(t, blockedState)
	destination := filepath.Join(directory, "destination")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "child"), []byte(strings.Repeat("x", 2)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(destination, map[string]string{}); err == nil {
		t.Fatal("state replaced a non-empty directory")
	}
	if data, err := os.ReadFile(filepath.Join(destination, "child")); err != nil || string(data) != "xx" {
		t.Fatalf("rename failure changed destination: %q, %v", data, err)
	}
}

func TestSaveMarkerStateFileFailuresDoNotCommit(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		configure   func(*failingMarkerStateFile)
		wantChmod   bool
		wantFailure error
	}{
		"write": {configure: func(file *failingMarkerStateFile) { file.writeErr = os.ErrPermission }, wantFailure: os.ErrPermission},
		"chmod": {configure: func(file *failingMarkerStateFile) { file.chmodErr = os.ErrPermission }, wantChmod: true, wantFailure: os.ErrPermission},
		"close": {configure: func(file *failingMarkerStateFile) { file.closeErr = os.ErrClosed }, wantChmod: true, wantFailure: os.ErrClosed},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "state.json")
			file := &failingMarkerStateFile{name: filepath.Join(t.TempDir(), "temporary")}
			test.configure(file)
			err := saveJSONWithFile(destination, map[string]string{"value": "ok"}, func(string) (markerStateFile, error) { return file, nil })
			if !errors.Is(err, test.wantFailure) {
				t.Fatalf("save error = %v, want %v", err, test.wantFailure)
			}
			if !file.wrote || file.chmodded != test.wantChmod || !file.closeCall {
				t.Fatalf("file operations = write %v, chmod %v, close %v", file.wrote, file.chmodded, file.closeCall)
			}
			requireMissingMarkerState(t, destination)
		})
	}
}

func TestPersistedMarkerControlCharacters(t *testing.T) {
	t.Parallel()
	if !hasControl("bad\nvalue") || hasControl("clean") {
		t.Fatal("control character classification changed")
	}
	if validRecordID("bad/id") || validRecordID(strings.Repeat("i", 129)) {
		t.Fatal("invalid record identity was accepted")
	}
	if validPersistedMarker(Marker{Type: "intro", Label: "Intro", Start: 1, End: 2, Source: "unknown"}) {
		t.Fatal("unknown marker source was accepted")
	}
}
