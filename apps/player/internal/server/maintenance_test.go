package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type blockingResponseWriter struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
}

func (writer *blockingResponseWriter) Header() http.Header { return writer.header }
func (writer *blockingResponseWriter) WriteHeader(int)     {}
func (writer *blockingResponseWriter) Write(data []byte) (int, error) {
	select {
	case <-writer.started:
	default:
		close(writer.started)
	}
	<-writer.release
	return len(data), nil
}

func TestAutomaticMaintenanceDefersHeavyWorkWhileStreaming(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	cacheDir := t.TempDir()
	cacheFile := filepath.Join(cacheDir, "0000000000000001", "segment.m4s")
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: cacheDir, MaintenanceInterval: time.Hour, TranscodeCacheLimit: 1})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatal("home lacks a media link")
	}
	id := match[1]
	stream := &blockingResponseWriter{header: make(http.Header), started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(stream, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+id, nil))
		close(done)
	}()
	select {
	case <-stream.started:
	case <-time.After(time.Second):
		t.Fatal("media stream did not start")
	}

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/maintenance", nil))
	requireResponse(t, status, http.StatusOK, `"activity":"busy"`)
	run := httptest.NewRecorder()
	handler.ServeHTTP(run, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/maintain", nil))
	requireResponse(t, run, http.StatusNoContent)
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("active playback cache was pruned: %v", err)
	}
	close(stream.release)
	<-done
}

func TestAutomaticMaintenanceBoundsOnlyTheTranscodeCache(t *testing.T) { //nolint:cyclop // One lifecycle test verifies eviction, accounting, and download preservation.
	t.Parallel()

	cacheDir := t.TempDir()
	oldDirectory := filepath.Join(cacheDir, "0000000000000001")
	newDirectory := filepath.Join(cacheDir, "0000000000000002")
	download := filepath.Join(cacheDir, "downloads", "keep.mp4")
	for path, content := range map[string]string{
		filepath.Join(oldDirectory, "segment.m4s"): "old!",
		filepath.Join(newDirectory, "segment.m4s"): "new!",
		download: "offline",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldDirectory, old, old); err != nil {
		t.Fatal(err)
	}

	handler := server.New(server.Config{Lifecycle: t.Context(), DataDir: t.TempDir(), CacheDir: cacheDir, MaintenanceInterval: 10 * time.Millisecond, TranscodeCacheLimit: 5, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, oldErr := os.Stat(oldDirectory)
		_, newErr := os.Stat(newDirectory)
		_, downloadErr := os.Stat(download)
		status := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/maintenance", nil)
		if os.IsNotExist(oldErr) && newErr == nil && downloadErr == nil && strings.Contains(status.Body.String(), `"bytes":4,"limit":5`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("automatic maintenance did not safely bound the transcode cache")
}

func TestAutomaticMaintenanceStatusIsAvailableThroughAPIAndWeb(t *testing.T) {
	t.Parallel()

	// Status and explicit maintenance calls do not need background schedules.
	handler := server.New(server.Config{DataDir: t.TempDir(), CacheDir: t.TempDir(), BackupDir: t.TempDir(), BackupKey: strings.Repeat("a", 64), BackupInterval: time.Hour, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value

	status := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/maintenance", nil)
	runAPI := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/tasks/maintain", nil)
	runWeb := requestWithCookie(t, handler, http.MethodPost, "/settings/tasks/maintain", "", owner)
	settings := getWithCookie(t, handler, "/settings", owner)
	requireResponse(t, status, http.StatusOK, `"mode":"automatic"`, `"analysis":{"mode":"on-change"`, `"cache":{"mode":"bounded"`, `"backups":{"state":"automatic","encrypted":true`)
	requireResponse(t, runAPI, http.StatusNoContent)
	requireResponse(t, runWeb, http.StatusSeeOther)
	requireResponse(t, settings, http.StatusOK, "Kinosail maintains itself automatically", "Encrypted backups run automatically")
}

func TestOwnerCanDownloadBackupRevokeSessionsAndClearCache(t *testing.T) { //nolint:cyclop // One owner workflow verifies every explicit maintenance action.
	t.Parallel()

	dataDir, cacheDir := t.TempDir(), t.TempDir()
	cacheFile := filepath.Join(cacheDir, "0000000000000001", "segment.m4s")
	download := filepath.Join(cacheDir, "downloads", "keep.mp4")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(download), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(download, []byte("offline"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: dataDir, CacheDir: cacheDir, RequireAuth: true})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var result struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, setup, &result)
	assertAPIBody(t, apiCall(t, handler, result.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, result.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	owner := &http.Cookie{Name: "__Host-kinosail_session", Value: result.Token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	otherRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Owner&password=owner-password&code="+testTOTP(t, result.TOTP.Secret, time.Now())))
	otherRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	otherResponse := httptest.NewRecorder()
	handler.ServeHTTP(otherResponse, otherRequest)
	other := otherResponse.Result().Cookies()[0]

	backup := getWithCookie(t, handler, "/settings/backup", owner)
	postWithCookie(t, handler, "/settings/cache/clear", owner)
	postWithCookie(t, handler, "/settings/sessions/revoke", owner)
	current := getWithCookie(t, handler, "/settings", owner)
	revoked := getWithCookie(t, handler, "/", other)
	if backup.Code != http.StatusOK || backup.Header().Get("Content-Type") != "application/gzip" || len(backup.Body.Bytes()) < 2 || current.Code != http.StatusOK || revoked.Code != http.StatusSeeOther {
		t.Fatalf("backup = %d, current = %d, revoked = %d", backup.Code, current.Code, revoked.Code)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("cache still exists: %v", err)
	}
	if _, err := os.Stat(download); err != nil {
		t.Fatalf("offline download was removed: %v", err)
	}
}

func getWithCookie(t *testing.T, handler http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func postWithCookie(t *testing.T, handler http.Handler, path string, cookie *http.Cookie) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST %s = %d %q", path, response.Code, response.Body.String())
	}
}
