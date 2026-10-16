package backup

import (
	"bytes"
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func TestScheduledBackupReadsLiveStoreWhileAWriterHoldsTheLock(t *testing.T) {
	directory := t.TempDir()
	store, err := database.Open(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveJSON("settings.json", map[string]string{"name": "Live"}); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(directory, database.Filename), RawQuery: "_txlock=immediate"}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	held, err := writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := held.ExecContext(context.Background(), `UPDATE state SET value = value WHERE name = 'settings.json'`); err != nil {
		t.Fatal(err)
	}
	const key = "scheduled-backup-key"
	var archive bytes.Buffer
	started := time.Now()
	err = LiveWriteEncrypted(store)(&archive, directory, key)
	if err != nil || time.Since(started) > 2*time.Second {
		t.Fatalf("backup during held write lock = %v after %s", err, time.Since(started))
	}
	if err := held.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyAuto(bytes.NewReader(archive.Bytes()), key); err != nil {
		t.Fatal(err)
	}
	restored := t.TempDir()
	if err := service.RestoreAuto(bytes.NewReader(archive.Bytes()), restored, key); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(restored, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var settings map[string]string
	if found, err := reopened.LoadJSON("settings.json", &settings); err != nil || !found || settings["name"] != "Live" {
		t.Fatalf("restored settings = %#v, %t, %v", settings, found, err)
	}
	if err := store.SaveJSON("settings.json", map[string]string{"name": "After"}); err != nil {
		t.Fatalf("live store unusable after backup: %v", err)
	}
}
