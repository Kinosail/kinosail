package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAutomaticBackupsFailClosedWithoutEncryptionKey(t *testing.T) {
	servertest.AssertAutomaticBackupsFailClosed(t, backupAPIFixture())
}

func TestOwnerCanRunEncryptedBackupThroughAPIAndWeb(t *testing.T) {
	servertest.AssertEncryptedBackupAPIAndWeb(t, backupAPIFixture())
}

func backupAPIFixture() servertest.BackupAPIFixture {
	return servertest.BackupAPIFixture{
		New: func(config servertest.BackupAPIConfig) http.Handler {
			return server.New(server.Config{Lifecycle: config.Lifecycle, DataDir: config.DataDir, BackupDir: config.BackupDir, BackupKey: config.Key, BackupInterval: config.Interval, RequireAuth: true})
		},
		SignIn: signInTestProfile, Get: getWithCookie, Web: requestWithCookie,
	}
}

func TestPortableBackupRejectsInvalidLegacySettingsWithoutDeletingSource(t *testing.T) {
	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	original := []byte(`{"subtitleLanguages":[]}`)
	settingsPath := filepath.Join(dataDir, "settings.json")
	if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	response := getWithCookie(t, handler, "/settings/backup", owner)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Content-Type") == "application/gzip" {
		t.Fatalf("invalid backup response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	current, err := os.ReadFile(settingsPath)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("rejected backup changed source: %q, %v", current, err)
	}
}
