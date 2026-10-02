package backup_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

type recoveryManifestCase struct {
	name, manifest string
	valid          bool
}

func recoveryManifestCases() []recoveryManifestCase {
	base := `{"format":1,"kinosailVersion":"dev","stateSchema":1,"createdAt":"2026-10-02T00:00:00Z","files":["progress.json"],"mediaIncluded":false}`
	replace := func(old, next string) string { return strings.Replace(base, old, next, 1) }
	return []recoveryManifestCase{
		{"valid", base, true},
		{"escaped canonical key", replace(`"stateSchema":1`, `"state\u0053chema":1`), true},
		{"single escaped format key", replace(`"format":1`, `"\u0066ormat":1`), true},
		{"single ASCII format alias", replace(`"format":1`, `"FORMAT":1`), true},
		{"single Unicode long S alias", replace(`"stateSchema":1`, `"ſtateSchema":1`), true},
		{"single Unicode Kelvin alias", replace(`"kinosailVersion":"dev"`, `"KinosailVersion":"dev"`), true},
		{"unknown manifest field", replace(`"format":1`, `"format":1,"futureField":true`), false},
		{"conflicting format valid last", replace(`"format":1`, `"format":0,"format":1`), false},
		{"conflicting format invalid last", replace(`"format":1`, `"format":1,"format":0`), false},
		{"conflicting mediaIncluded valid last", replace(`"mediaIncluded":false`, `"mediaIncluded":true,"mediaIncluded":false`), false},
		{"conflicting mediaIncluded invalid last", replace(`"mediaIncluded":false`, `"mediaIncluded":false,"mediaIncluded":true`), false},
		{"equal canonical format", replace(`"format":1`, `"format":1,"format":1`), false},
		{"equal canonical mediaIncluded", replace(`"mediaIncluded":false`, `"mediaIncluded":false,"mediaIncluded":false`), false},
		{"escaped format alias", replace(`"format":1`, `"format":0,"\u0066ormat":1`), false},
		{"escaped mediaIncluded alias", replace(`"mediaIncluded":false`, `"mediaIncluded":true,"media\u0049ncluded":false`), false},
		{"ASCII format alias", replace(`"format":1`, `"FORMAT":0,"format":1`), false},
		{"Unicode long S alias valid last", replace(`"stateSchema":1`, `"ſtateSchema":0,"stateSchema":1`), false},
		{"Unicode long S alias invalid last", replace(`"stateSchema":1`, `"stateSchema":1,"ſtateSchema":0`), false},
		{"Unicode Kelvin alias", replace(`"kinosailVersion":"dev"`, `"KinosailVersion":"","kinosailVersion":"dev"`), false},
	}
}

func TestRestoreRejectsAmbiguousRecoveryManifest(t *testing.T) {
	for _, test := range recoveryManifestCases() {
		t.Run(test.name, func(t *testing.T) {
			destination := t.TempDir()
			for _, name := range []string{"progress.json", "settings.json", "kinosail.db", "kinosail.db-wal", "kinosail.db-shm"} {
				writeBackupFile(t, filepath.Join(destination, name), `{"previous":"`+name+`"}`)
			}
			before := recoveryDirectory(t, destination)
			databaseOpens := 0
			service := sharedbackup.MustNew(sharedbackup.Config{
				DatabaseFilename: "kinosail.db",
				OpenDatabase: func(string) (sharedbackup.Database, error) {
					databaseOpens++
					return nil, errors.New("unexpected test database")
				},
				ValidateSettings: func([]byte) error { return nil },
			})
			archive := tarArchive(t, []archiveEntry{
				{name: "manifest.json", data: test.manifest},
				{name: "progress.json", data: `{"item":{"seconds":42}}`},
			})
			err := service.Restore(bytes.NewReader(archive), destination)
			after := recoveryDirectory(t, destination)
			assertRecoveryManifestOutcome(t, test.valid, err, before, after)
			if !test.valid && databaseOpens != 0 {
				t.Errorf("rejected manifest opened database adapter %d times", databaseOpens)
			}
		})
	}
}

func assertRecoveryManifestOutcome(t *testing.T, valid bool, err error, before, after map[string]string) {
	t.Helper()
	if valid {
		if err != nil {
			t.Fatalf("valid manifest could not restore: %v", err)
		}
		want := map[string]string{"progress.json": `{"item":{"seconds":42}}`}
		if !reflect.DeepEqual(after, want) {
			t.Fatalf("valid restore state = %v, want %v", after, want)
		}
		return
	}
	if err == nil {
		t.Errorf("ambiguous manifest was accepted")
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("rejected manifest changed installation state: before=%v after=%v", before, after)
	}
}

func recoveryDirectory(t *testing.T, directory string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	state := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected restore directory %s", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		state[entry.Name()] = string(data)
	}
	return state
}
