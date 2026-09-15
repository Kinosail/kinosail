package documentdb

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var errInjected = errors.New("injected failure")

func TestOpenContextSurfacesDependencyFailures(t *testing.T) { //nolint:cyclop // The table covers each injected operating-system boundary.
	cases := map[string]struct {
		legacy bool
		change func(*dependencies)
	}{
		"mkdir": {change: func(value *dependencies) {
			value.mkdirAll = func(string, fs.FileMode) error { return errInjected }
		}},
		"stat": {change: func(value *dependencies) {
			value.lstat = func(string) (fs.FileInfo, error) { return nil, errInjected }
		}},
		"open": {change: func(value *dependencies) {
			value.openDB = func(string, string) (*sql.DB, error) { return nil, errInjected }
		}},
		"chmod": {change: func(value *dependencies) {
			value.chmod = func(string, fs.FileMode) error { return errInjected }
		}},
		"remove": {legacy: true, change: func(value *dependencies) {
			value.remove = func(string) error { return errInjected }
		}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if test.legacy {
				if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{}`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			deps := systemDependencies()
			test.change(&deps)
			store, err := openContext(t.Context(), directory, false, testConfig, deps)
			if store != nil || !errors.Is(err, errInjected) {
				t.Fatalf("openContext() = %#v, %v", store, err)
			}
		})
	}
}

func TestOpenContextCoversConnectionAndExportPolicies(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, true, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	raw := openRaw(t, filepath.Join(directory, Filename))
	if _, err := raw.ExecContext(t.Context(), `INSERT INTO state(name, value) VALUES('unknown.json', ?)`, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	if reopened, err := Open(directory, false, testConfig); err == nil || reopened != nil {
		t.Fatalf("invalid export state = %#v, %v", reopened, err)
	}
}

func TestStoreSurfacesDecodeFailuresAndMissingDocuments(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Load("settings.json"); err != nil || found {
		t.Fatalf("missing document = %v, %v", found, err)
	}
	if err := store.SaveJSON("settings.json", "wrong type"); err != nil {
		t.Fatal(err)
	}
	if found, err := store.LoadJSON("settings.json", &map[string]string{}); err == nil || found {
		t.Fatalf("type mismatch = %v, %v", found, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreSurfacesClosedDatabaseErrors(t *testing.T) {
	store, err := Open(t.TempDir(), false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load("settings.json"); err == nil {
		t.Fatal("closed load was accepted")
	}
	if err := store.SaveJSON("settings.json", true); err == nil {
		t.Fatal("closed save was accepted")
	}
	if _, err := store.Export(); err == nil {
		t.Fatal("closed export was accepted")
	}
}

func TestValidateExistingSurfacesDatabaseFailures(t *testing.T) {
	for name, configure := range map[string]func(sqlmock.Sqlmock){
		"query": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(selectStatePattern()).WillReturnError(errInjected)
		},
		"scan": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(selectStatePattern()).WillReturnRows(sqlmock.NewRows([]string{"name", "value"}).AddRow(nil, []byte(`{}`)))
		},
		"rows": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(selectStatePattern()).WillReturnRows(sqlmock.NewRows([]string{"name", "value"}).AddRow("settings.json", []byte(`{}`)).RowError(0, errInjected))
		},
	} {
		t.Run(name, func(t *testing.T) {
			store, mock, closeStore := mockStore(t)
			defer closeStore()
			expectVersion(mock, 1)
			configure(mock)
			if err := store.validateExisting(t.Context()); err == nil {
				t.Fatal("database failure was accepted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitializeSurfacesDatabaseFailures(t *testing.T) { //nolint:cyclop,funlen // The table covers each transaction stage without production side effects.
	for name, configure := range map[string]func(sqlmock.Sqlmock){
		"version": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(versionPattern()).WillReturnError(errInjected)
		},
		"pragmas": func(mock sqlmock.Sqlmock) {
			expectVersion(mock, 1)
			mock.ExpectExec(pragmaPattern()).WillReturnError(errInjected)
		},
		"schema": func(mock sqlmock.Sqlmock) {
			expectVersion(mock, 0)
			mock.ExpectExec(pragmaPattern()).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec(schemaPattern()).WillReturnError(errInjected)
		},
		"quick query": func(mock sqlmock.Sqlmock) {
			expectBeforeQuickCheck(mock, 1)
			mock.ExpectQuery(quickCheckPattern()).WillReturnError(errInjected)
		},
		"quick result": func(mock sqlmock.Sqlmock) {
			expectBeforeQuickCheck(mock, 1)
			mock.ExpectQuery(quickCheckPattern()).WillReturnRows(sqlmock.NewRows([]string{"quick_check"}).AddRow("bad"))
		},
		"begin": func(mock sqlmock.Sqlmock) {
			expectHealthyDatabase(mock)
			mock.ExpectBegin().WillReturnError(errInjected)
		},
		"retired cleanup": func(mock sqlmock.Sqlmock) {
			expectHealthyDatabase(mock)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM state_entries WHERE name = ?`)).WithArgs("live-tv.json").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM state WHERE name = ?`)).WithArgs("live-tv.json").WillReturnError(errInjected)
			mock.ExpectRollback()
		},
		"legacy import": func(mock sqlmock.Sqlmock) {
			expectHealthyDatabase(mock)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM state_entries WHERE name = ?`)).WithArgs("live-tv.json").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM state WHERE name = ?`)).WithArgs("live-tv.json").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT entry_mode FROM state WHERE name = ?`)).WithArgs("settings.json").WillReturnRows(sqlmock.NewRows([]string{"entry_mode"}))
			mock.ExpectExec("INSERT INTO state").WithArgs("settings.json", []byte(`{}`)).WillReturnError(errInjected)
			mock.ExpectRollback()
		},
	} {
		t.Run(name, func(t *testing.T) {
			store, mock, closeStore := mockStore(t)
			defer closeStore()
			configure(mock)
			legacy := map[string][]byte(nil)
			if name == "legacy import" {
				legacy = map[string][]byte{"settings.json": []byte(`{}`)}
			}
			if err := store.initialize(t.Context(), legacy); err == nil {
				t.Fatal("database failure was accepted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func mockStore(t *testing.T) (*Store, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	config, allowed, err := validateConfig(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	return &Store{db: db, config: config, allowed: allowed}, mock, func() { _ = db.Close() }
}

func expectVersion(mock sqlmock.Sqlmock, version int) {
	mock.ExpectQuery(versionPattern()).WillReturnRows(sqlmock.NewRows([]string{"user_version"}).AddRow(version))
}

func expectBeforeQuickCheck(mock sqlmock.Sqlmock, version int) {
	expectVersion(mock, version)
	mock.ExpectExec(pragmaPattern()).WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectHealthyDatabase(mock sqlmock.Sqlmock) {
	expectBeforeQuickCheck(mock, SchemaVersion)
	mock.ExpectQuery(quickCheckPattern()).WillReturnRows(sqlmock.NewRows([]string{"quick_check"}).AddRow("ok"))
}

func versionPattern() string { return regexp.QuoteMeta(`PRAGMA user_version`) }
func pragmaPattern() string {
	return regexp.QuoteMeta(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;`)
}
func schemaPattern() string     { return "CREATE TABLE state" }
func quickCheckPattern() string { return regexp.QuoteMeta(`PRAGMA quick_check`) }
func selectStatePattern() string {
	return regexp.QuoteMeta(`SELECT name, value FROM state ORDER BY name`)
}
