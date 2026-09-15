package documentdb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testConfig = Config{
	Documents:        []string{"profiles.json", "settings.json"},
	RetiredDocuments: []string{"live-tv.json"},
}

func TestStoreLifecycleAndLegacyMigration(t *testing.T) { //nolint:cyclop // One lifecycle test verifies the related durability contract.
	directory := t.TempDir()
	legacy := filepath.Join(directory, "settings.json")
	if err := os.WriteFile(legacy, []byte(`{"name":"Home"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var settings map[string]string
	if found, loadErr := store.LoadJSON("settings.json", &settings); loadErr != nil || !found || settings["name"] != "Home" {
		t.Fatalf("LoadJSON() = %#v, %v, %v", settings, found, loadErr)
	}
	if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy state remains: %v", err)
	}
	if info, err := os.Stat(filepath.Join(directory, Filename)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("database permissions = %v, %v", info, err)
	}
	if err := store.SaveJSONBatch(map[string]any{
		"settings.json": map[string]string{"name": "Updated"},
		"profiles.json": map[string]string{"owner": "Captain"},
	}); err != nil {
		t.Fatal(err)
	}
	exported, err := store.Export()
	if err != nil || len(exported) != 2 {
		t.Fatalf("Export() = %#v, %v", exported, err)
	}
	exported["settings.json"][0] = 'x'
	if _, found, err := store.Load("settings.json"); err != nil || !found {
		t.Fatalf("export mutated stored state: %v, %v", found, err)
	}
}

func TestOpenRejectsInvalidConfigBeforeSideEffects(t *testing.T) {
	tooMany := make([]string, maxDocuments+1)
	for index := range tooMany {
		tooMany[index] = string(rune('a'+index%26)) + strings.Repeat("a", index/26) + ".json"
	}
	for name, config := range map[string]Config{
		"empty":        {},
		"too many":     {Documents: tooMany},
		"empty name":   {Documents: []string{""}},
		"wrong suffix": {Documents: []string{"state.txt"}},
		"traversal":    {Documents: []string{"../state.json"}},
		"backslash":    {Documents: []string{`..\state.json`}},
		"whitespace":   {Documents: []string{" state.json"}},
		"duplicate":    {Documents: []string{"state.json", "state.json"}},
		"retired clash": {
			Documents: []string{"state.json"}, RetiredDocuments: []string{"state.json"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "state")
			if store, err := Open(directory, false, config); err == nil || store != nil {
				t.Fatalf("Open() = %#v, %v", store, err)
			}
			if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid config changed disk: %v", err)
			}
		})
	}
}

func TestPlayerConfigReturnsIndependentDocuments(t *testing.T) {
	first := PlayerConfig()
	first.Documents[0] = "changed.json"
	if second := PlayerConfig(); second.Documents[0] == "changed.json" {
		t.Fatal("PlayerConfig shared mutable state")
	}
}

func TestOpenRejectsInvalidLegacyStateWithoutSideEffects(t *testing.T) { //nolint:gocognit // The score of 16 remains below the repository ceiling of 22 for the invalid-state matrix.
	validator := func(_ string, data []byte) error {
		if strings.Contains(string(data), "reject") {
			return errors.New("rejected")
		}
		return nil
	}
	for name, setup := range map[string]func(string) error{
		"malformed": func(path string) error { return os.WriteFile(path, []byte("{"), 0o600) },
		"semantic":  func(path string) error { return os.WriteFile(path, []byte(`{"reject":true}`), 0o600) },
		"oversized": writeOversizedLegacy,
		"symlink":   writeSymlinkLegacy,
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			legacy := filepath.Join(directory, "settings.json")
			if err := setup(legacy); err != nil {
				t.Fatal(err)
			}
			config := testConfig
			config.Validate = validator
			if store, err := Open(directory, false, config); err == nil || store != nil {
				t.Fatalf("Open() = %#v, %v", store, err)
			}
			if _, err := os.Stat(filepath.Join(directory, Filename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("database created after rejection: %v", err)
			}
			if _, err := os.Lstat(legacy); err != nil {
				t.Fatalf("legacy state changed after rejection: %v", err)
			}
		})
	}
}

func TestOpenRejectsInvalidDatabaseState(t *testing.T) { //nolint:cyclop // Cases cover distinct database trust boundaries.
	for name, prepare := range map[string]func(*testing.T, string){
		"directory": func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"corrupt": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("not SQLite"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"future schema": func(t *testing.T, path string) {
			raw := openRaw(t, path)
			if _, err := raw.ExecContext(t.Context(), `PRAGMA user_version=2`); err != nil {
				t.Fatal(err)
			}
			_ = raw.Close()
		},
		"missing table": func(t *testing.T, path string) {
			raw := openRaw(t, path)
			if _, err := raw.ExecContext(t.Context(), `PRAGMA user_version=1`); err != nil {
				t.Fatal(err)
			}
			_ = raw.Close()
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			prepare(t, filepath.Join(directory, Filename))
			if store, err := Open(directory, false, testConfig); err == nil || store != nil {
				t.Fatalf("Open() = %#v, %v", store, err)
			}
		})
	}
}

func TestStoreRejectsInvalidOperationsWithoutWriting(t *testing.T) { //nolint:cyclop // Each assertion covers one public input boundary.
	store, err := Open(t.TempDir(), false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, operation := range []func() error{
		func() error { return store.SaveJSON("unknown.json", true) },
		func() error { return store.SaveJSON("settings.json", make(chan bool)) },
		func() error { return store.SaveJSONBatchContext(nilContext, map[string]any{"settings.json": true}) },
		func() error {
			return store.SaveJSONBatch(map[string]any{"settings.json": true, "unknown.json": true})
		},
	} {
		if err := operation(); err == nil {
			t.Fatal("invalid operation was accepted")
		}
	}
	if _, _, err := store.Load("unknown.json"); err == nil {
		t.Fatal("unknown document was loaded")
	}
	if _, err := store.ExportContext(nilContext); err == nil {
		t.Fatal("nil context was accepted")
	}
	if exported, err := store.Export(); err != nil || len(exported) != 0 {
		t.Fatalf("rejected operations wrote state: %#v, %v", exported, err)
	}
}

func TestBatchRollbackAndPersistedValidation(t *testing.T) { //nolint:cyclop // One test verifies the transaction and validation boundaries together.
	directory := t.TempDir()
	store, err := Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	before := map[string]any{"settings.json": map[string]string{"name": "Before"}, "profiles.json": map[string]string{"owner": "Before"}}
	if err := store.SaveJSONBatch(before); err != nil {
		t.Fatal(err)
	}
	raw := openRaw(t, filepath.Join(directory, Filename))
	defer raw.Close()
	if _, err := raw.ExecContext(t.Context(), `CREATE TRIGGER fail_profiles BEFORE UPDATE ON state
		WHEN NEW.name = 'profiles.json' BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSONBatch(map[string]any{"settings.json": map[string]string{"name": "After"}, "profiles.json": map[string]string{"owner": "After"}}); err == nil {
		t.Fatal("transaction failure was accepted")
	}
	var settings map[string]string
	if found, err := store.LoadJSON("settings.json", &settings); err != nil || !found || settings["name"] != "Before" {
		t.Fatalf("rollback state = %#v, %v, %v", settings, found, err)
	}
	if _, err := raw.ExecContext(t.Context(), `DROP TRIGGER fail_profiles`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(t.Context(), `UPDATE state SET value = ? WHERE name = 'settings.json'`, []byte("{")); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Load("settings.json"); err == nil || found {
		t.Fatalf("malformed state = %v, %v", found, err)
	}
	if _, err := store.Export(); err == nil {
		t.Fatal("malformed export was accepted")
	}
}

func TestValidationBeforeMigrationPreventsCleanup(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	raw := openRaw(t, filepath.Join(directory, Filename))
	if _, err := raw.ExecContext(t.Context(), `INSERT INTO state(name, value) VALUES('settings.json', ?), ('live-tv.json', ?)`, []byte(`{"reject":true}`), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	config := testConfig
	config.ValidateBeforeMigration = true
	config.Validate = func(_ string, data []byte) error {
		if strings.Contains(string(data), "reject") {
			return errors.New("rejected")
		}
		return nil
	}
	if reopened, err := Open(directory, false, config); err == nil || reopened != nil {
		t.Fatalf("Open() = %#v, %v", reopened, err)
	}
	raw = openRaw(t, filepath.Join(directory, Filename))
	defer raw.Close()
	var retained int
	if err := raw.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM state WHERE name = 'live-tv.json'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("rejected migration changed retired state: %d, %v", retained, err)
	}
}

func TestNilStoreAndEmptyDirectoryAreNoOps(t *testing.T) { //nolint:cyclop // One test covers all nil-safe operations.
	var store *Store
	if opened, err := Open("", false, Config{}); err != nil || opened != nil {
		t.Fatalf("Open empty = %#v, %v", opened, err)
	}
	if store.Close() != nil || store.SaveJSON("settings.json", nil) != nil || store.SaveJSONBatch(nil) != nil {
		t.Fatal("nil store operation failed")
	}
	if data, found, err := store.Load("settings.json"); data != nil || found || err != nil {
		t.Fatalf("nil Load() = %q, %v, %v", data, found, err)
	}
	if found, err := store.LoadJSON("settings.json", new(any)); found || err != nil {
		t.Fatalf("nil LoadJSON() = %v, %v", found, err)
	}
	if exported, err := store.Export(); err != nil || len(exported) != 0 {
		t.Fatalf("nil Export() = %#v, %v", exported, err)
	}
	if opened, err := OpenContext(nilContext, t.TempDir(), false, testConfig); err == nil || opened != nil {
		t.Fatalf("nil-context Open() = %#v, %v", opened, err)
	}
}
