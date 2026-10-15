package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestAutomaticMaintenanceStatusIsAvailableThroughAPI(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{DataDir: t.TempDir(), CacheDir: t.TempDir(), BackupDir: t.TempDir(), BackupKey: strings.Repeat("a", 64), BackupInterval: time.Hour, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value

	status := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/maintenance", nil)
	runAPI := apiCall(t, handler, session.Token, http.MethodPost, "/api/v1/tasks/maintain", nil)
	runWeb := requestWithCookie(t, handler, http.MethodPost, "/settings/tasks/maintain", "", owner)
	requireResponse(t, status, http.StatusOK, `"mode":"automatic"`, `"backups":{"state":"automatic","encrypted":true`)
	requireResponse(t, runAPI, http.StatusNoContent)
	requireResponse(t, runWeb, http.StatusSeeOther)
}

func TestOwnerCanDownloadBackupAndRevokeSessions(t *testing.T) {
	t.Parallel()

	dataDir, cacheDir := t.TempDir(), t.TempDir()
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
	postWithCookie(t, handler, "/settings/sessions/revoke", owner)
	current := getWithCookie(t, handler, "/settings", owner)
	revoked := getWithCookie(t, handler, "/", other)
	if backup.Code != http.StatusOK || backup.Header().Get("Content-Type") != "application/gzip" || len(backup.Body.Bytes()) < 2 || current.Code != http.StatusOK || revoked.Code != http.StatusSeeOther {
		t.Fatalf("backup = %d, current = %d, revoked = %d", backup.Code, current.Code, revoked.Code)
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
