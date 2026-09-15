package servertest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// BackupAPIConfig supplies the original encrypted and automatic backup fixtures.
type BackupAPIConfig struct {
	Lifecycle               context.Context
	DataDir, BackupDir, Key string
	Interval                time.Duration
}

// BackupAPIFixture binds real application handlers and their existing cookie helpers.
type BackupAPIFixture struct {
	New    func(BackupAPIConfig) http.Handler
	SignIn func(*testing.T, http.Handler, string, string) *http.Cookie
	Get    func(*testing.T, http.Handler, string, *http.Cookie) *httptest.ResponseRecorder
	Web    func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
}

// AssertAutomaticBackupsFailClosed preserves the no-key API and web regression.
func AssertAutomaticBackupsFailClosed(t *testing.T, fixture BackupAPIFixture) {
	t.Helper()
	t.Parallel()
	backupDir := t.TempDir()
	handler := fixture.New(BackupAPIConfig{Lifecycle: t.Context(), DataDir: t.TempDir(), BackupDir: backupDir, Interval: 20 * time.Millisecond})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/backups", nil)
		if strings.Contains(status.Body.String(), `"lastError":"backup encryption key is required"`) {
			assertBackupDisabled(t, backupDir, status)
			page := fixture.Get(t, handler, "/settings/backups", owner)
			AssertAPIBody(t, page, http.StatusOK, "Automatic backups", "Backups need setup.", `href="/settings/configuration#backup.key"`)
			if !strings.Contains(page.Body.String(), `<button disabled>Back up now</button>`) || !strings.Contains(page.Body.String(), `<button disabled>Verify latest backup</button>`) {
				t.Fatalf("unconfigured backup actions = %q", page.Body.String())
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("missing encryption key was not reported")
}

func assertBackupDisabled(t *testing.T, backupDir string, status *httptest.ResponseRecorder) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(backupDir, "kinosail-*"))
	if err != nil || len(files) != 0 || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("unencrypted backup files=%v status=%q err=%v", files, status.Body.String(), err)
	}
}

// AssertEncryptedBackupAPIAndWeb checks both adapters and the encrypted file format.
func AssertEncryptedBackupAPIAndWeb(t *testing.T, fixture BackupAPIFixture) {
	t.Helper()
	backupDir := t.TempDir()
	handler := fixture.New(BackupAPIConfig{DataDir: t.TempDir(), BackupDir: backupDir, Key: "container-test-backup-key"})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	api := APICall(t, handler, owner.Value, http.MethodPost, "/api/v1/backups", nil)
	verified := APICall(t, handler, owner.Value, http.MethodPost, "/api/v1/backups/verify", nil)
	status := APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/backups", nil)
	web := fixture.Web(t, handler, http.MethodPost, "/settings/encrypted-backup", "", owner)
	files, err := filepath.Glob(filepath.Join(backupDir, "kinosail-*.backup"))
	if err != nil || api.Code != http.StatusNoContent || verified.Code != http.StatusNoContent || status.Code != http.StatusOK || web.Code != http.StatusSeeOther || len(files) == 0 {
		t.Fatalf("api=%d verify=%d status=%d %q web=%d files=%v err=%v", api.Code, verified.Code, status.Code, status.Body.String(), web.Code, files, err)
	}
	assertEncryptedBackupFile(t, files[len(files)-1], status)
	page := fixture.Get(t, handler, "/settings/backups", owner)
	AssertAPIBody(t, page, http.StatusOK, "Automatic backups", "Encryption is on.", `href="/settings/configuration#backup.key"`)
}

func assertEncryptedBackupFile(t *testing.T, path string, status *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(status.Body.String(), `"enabled":true`) || !strings.Contains(status.Body.String(), `"lastVerified"`) {
		t.Fatalf("backup status = %q", status.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte("KINOSAIL-BACKUP-1\n")) || !strings.Contains(status.Body.String(), `"encrypted":true`) {
		t.Fatalf("encrypted backup status=%q err=%v", status.Body.String(), err)
	}
}
