package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// AssertNewInstallationCreatesOwnerProfile preserves the original real-handler authentication regression.
func AssertNewInstallationCreatesOwnerProfile(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup" {
		t.Fatalf("anonymous home = %d %q", response.Code, response.Header().Get("Location"))
	}
	ownerCookie := fixture.SignIn(t, handler, "/setup", "name=Mike&password=correct+horse+battery+staple")
	handler = fixture.NewHandler("", dataDir, true)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(ownerCookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("owner home after restart = %d", response.Code)
	}
	assertOneYearBrowserSessionAfterRestart(t, fixture, handler, ownerCookie, dataDir)
	handler = fixture.NewHandler("", dataDir, true)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Mike&password=correct+horse+battery+staple"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	profiles := fixture.StoredState(t, dataDir, "profiles.json")
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "authenticator app") || strings.Contains(string(profiles), "correct horse") {
		t.Fatalf("login = %d %q, profiles = %q", response.Code, response.Body.String(), profiles)
	}
}

func assertOneYearBrowserSessionAfterRestart(t *testing.T, fixture LibraryAPIFixture, handler http.Handler, ownerCookie *http.Cookie, dataDir string) {
	t.Helper()
	AssertAPIBody(t, APICall(t, handler, ownerCookie.Value, http.MethodPut, "/api/v1/settings/session-timeouts", map[string]any{"inactiveHours": 8760, "absoluteHours": 8760}), http.StatusOK, `"status":"saved"`)
	var enrolled []struct{ TOTPSecret string }
	if err := json.Unmarshal(fixture.StoredState(t, dataDir, "profiles.json"), &enrolled); err != nil || len(enrolled) != 1 {
		t.Fatalf("owner enrollment = %v, count = %d", err, len(enrolled))
	}
	ownerCookie = fixture.SignIn(t, handler, "/login", "name=Mike&password=correct+horse+battery+staple&code="+TestTOTP(t, enrolled[0].TOTPSecret, time.Now()))
	if ownerCookie.MaxAge != 365*24*60*60 || time.Until(ownerCookie.Expires) < 365*24*time.Hour-time.Minute {
		t.Fatalf("one-year browser cookie lifetime = %d, expiry = %v", ownerCookie.MaxAge, ownerCookie.Expires)
	}
	handler = fixture.NewHandler("", dataDir, true)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(ownerCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("one-year browser session after restart = %d", response.Code)
	}
}

// AssertViewerCanSignOut preserves the original real-handler authentication regression.
func AssertViewerCanSignOut(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/logout", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	handler = fixture.NewHandler("", dataDir, true)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("signed-out home = %d %q", response.Code, response.Header().Get("Location"))
	}
}

// AssertRepeatedLoginAttemptsAreLimited preserves the original real-handler authentication regression.
func AssertRepeatedLoginAttemptsAreLimited(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	_ = fixture.SignIn(t, handler, "/setup", "name=Owner&password=correct-password")
	var response *httptest.ResponseRecorder
	for range 11 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Owner&password=wrong-password"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.RemoteAddr = "192.0.2.1:1234"
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("eleventh login = %d", response.Code)
	}
}
