package servertest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// AssertLibraryKeyContract verifies that a browse-only key cannot disclose playback internals.
func AssertLibraryKeyContract(t *testing.T, handler http.Handler, library *httptest.ResponseRecorder, secret string) string {
	t.Helper()
	if library.Code != http.StatusOK || strings.Contains(library.Body.String(), `"download"`) || strings.Contains(library.Body.String(), `"stream"`) {
		t.Fatalf("library scope = %d %q", library.Code, library.Body.String())
	}
	id := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(library.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, request)
	if playback.Code != http.StatusOK || strings.Contains(playback.Body.String(), `"compatible"`) || strings.Contains(playback.Body.String(), `"direct"`) || strings.Contains(playback.Body.String(), `"trickplay"`) || strings.Contains(playback.Body.String(), `"source"`) {
		t.Fatalf("library scope playback = %d %q", playback.Code, playback.Body.String())
	}
	return id
}

// AssertLibraryKeyCannotMutate verifies that browse scope cannot change library state.
func AssertLibraryKeyCannotMutate(t *testing.T, handler http.Handler, id, secret string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/items/"+id+"/list", strings.NewReader(`{"listed":true}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("browse key mutation = %d %q", response.Code, response.Body.String())
	}
}

// RemovingProfileRevokesItsAPIKeys verifies that credentials cannot outlive their profile.
func (fixture LibraryAPIFixture) RemovingProfileRevokesItsAPIKeys(t *testing.T) {
	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	primary := fixture.SignIn(t, handler, "/setup", "name=Primary&password=primary-password")
	created := cookieForm(t, handler, http.MethodPost, "/settings/api-keys", "name=PrimaryKey&scopes=library", primary)
	secret := regexp.MustCompile(`ks_[A-Z2-7]+`).FindString(created.Body.String())
	cookieForm(t, handler, http.MethodPost, "/settings/profiles", "name=Second&password=second-password&owner=true", primary)
	removed := cookieForm(t, handler, http.MethodPost, "/settings/profiles/remove", "id="+fixture.StoredProfileID(t, dataDir, "Primary"), primary)
	if removed.Code != http.StatusSeeOther {
		t.Fatalf("remove profile = %d %q", removed.Code, removed.Body.String())
	}
	revoked := bearerGet(t, handler, "/api/v1/library", secret)
	stored := fixture.StoredState(t, dataDir, "api_keys.json")
	if revoked.Code != http.StatusUnauthorized || strings.Contains(string(stored), "PrimaryKey") {
		t.Fatalf("removed profile key: response=%d stored=%q", revoked.Code, stored)
	}
}

func cookieForm(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "http://kinosail.test"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func bearerGet(t *testing.T, handler http.Handler, path, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
