package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSQLiteBackupContract(t *testing.T) {
	servertest.SQLiteBackupContract(t, servertest.SQLiteBackupFixture[*database.Store]{
		Filename: database.Filename, Open: database.Open, Write: Write, Restore: service.Restore,
	})
}

func TestBackupRejectsInvalidLegacySettingsBeforeSQLiteOrArchiveEffects(t *testing.T) { //nolint:cyclop // One rejection contract proves every installation file remains unchanged.
	directory := t.TempDir()
	store, err := database.Open(directory, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(directory, database.Filename)
	databaseBefore, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"name":"Home","subtitleLanguages":[]}`)
	settingsPath := filepath.Join(directory, "settings.json")
	if err = os.WriteFile(settingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err = Write(&archive, directory); err == nil || archive.Len() != 0 {
		t.Fatalf("invalid legacy backup = %d bytes, %v", archive.Len(), err)
	}
	settingsAfter, settingsErr := os.ReadFile(settingsPath)
	databaseAfter, databaseErr := os.ReadFile(databasePath)
	if settingsErr != nil || !bytes.Equal(settingsAfter, original) {
		t.Fatalf("rejected backup changed legacy settings: %q, %v", settingsAfter, settingsErr)
	}
	if databaseErr != nil || !bytes.Equal(databaseAfter, databaseBefore) {
		t.Fatalf("rejected backup changed database: equal=%v error=%v", bytes.Equal(databaseAfter, databaseBefore), databaseErr)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err = os.Lstat(databasePath + suffix); !os.IsNotExist(err) {
			t.Fatalf("rejected backup created SQLite%s: %v", suffix, err)
		}
	}
}
