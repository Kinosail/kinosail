package server

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestScheduledBackupReadsTheLiveStoreWhileAWriterHoldsTheLock(t *testing.T) {
	dataDir, backupDir := t.TempDir(), t.TempDir()
	stateDB, err := database.Open(dataDir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer stateDB.Close()
	if err := stateDB.SaveJSON("sessions.json", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(dataDir, database.Filename), RawQuery: "_txlock=immediate"}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	held, err := writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Rollback() }()
	if _, err := held.ExecContext(context.Background(), `UPDATE state SET value = value WHERE name = 'sessions.json'`); err != nil {
		t.Fatal(err)
	}
	manager := newBackupManager(t.Context(), dataDir, backupDir, "container-test-backup-key", time.Hour, 7, workload.New(1), stateDB)
	started := time.Now()
	if err := manager.WriteNow(); err != nil || time.Since(started) > 2*time.Second {
		t.Fatalf("scheduled backup during held write lock = %v after %s", err, time.Since(started))
	}
	files, err := manager.Files()
	if err != nil || len(files) != 1 {
		t.Fatalf("published backups = %v, %v", files, err)
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatal(err)
	}
}
