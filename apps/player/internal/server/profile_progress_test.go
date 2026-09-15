package server_test

import (
	"encoding/json"
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

func TestViewerProfilesKeepSeparatePlaybackProgress(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	home := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(home, request)
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=60"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	viewer := signInTestProfile(t, handler, "/login", "name=Sam&password=viewer-password")
	home = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(viewer)
	handler.ServeHTTP(home, request)

	if strings.Contains(home.Body.String(), "Continue watching") {
		t.Fatalf("viewer inherited Owner progress: %q", home.Body.String())
	}
}

func TestViewerProfilesKeepSeparatePersistentLists(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	viewer := signInTestProfile(t, handler, "/login", "name=Sam&password=viewer-password")
	home := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(home, request)
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/list/"+id, strings.NewReader("listed=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	handler = server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	for name, expected := range map[string]struct {
		cookie *http.Cookie
		want   bool
	}{"owner": {owner, true}, "viewer": {viewer, false}} {
		response := httptest.NewRecorder()
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.AddCookie(expected.cookie)
		handler.ServeHTTP(response, request)
		if strings.Contains(response.Body.String(), "<h2>My List</h2>") != expected.want {
			t.Fatalf("%s home = %q", name, response.Body.String())
		}
	}
}

func TestViewerCanDismissContinueWatchingWithoutDeletingHistory(t *testing.T) {
	t.Parallel()

	handler, token := apiServer(t)
	libraryResponse := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(libraryResponse.Body.Bytes(), &catalog); err != nil || len(catalog.Items) == 0 {
		t.Fatalf("library = %d %q: %v", libraryResponse.Code, libraryResponse.Body.String(), err)
	}
	id := catalog.Items[0].ID
	assertAPICalls(t, handler, token, []apiTestCall{{Method: http.MethodPut, Path: "/api/v1/items/" + id + "/progress", Body: map[string]any{"seconds": 61}, Status: http.StatusOK}})
	home := apiCall(t, handler, token, http.MethodGet, "/", nil)
	assertAPIBody(t, home, http.StatusOK, `action="/continue-watching/`+id+`/remove"`, "Remove Arrival from Continue Watching")
	spanishHome := apiCall(t, handler, token, http.MethodGet, "/?lang=es", nil)
	assertAPIBody(t, spanishHome, http.StatusOK, "Eliminar Arrival de Seguir viendo")
	assertAPICalls(t, handler, token, []apiTestCall{{Method: http.MethodDelete, Path: "/api/v1/items/" + id + "/continue-watching", Body: nil, Status: http.StatusNoContent}})
	assertAPICalls(t, handler, token, []apiTestCall{{Method: http.MethodPut, Path: "/api/v1/items/" + id + "/progress", Body: map[string]any{"seconds": 61}, Status: http.StatusOK}})
	removed := apiCall(t, handler, token, http.MethodPost, "/continue-watching/"+id+"/remove", nil)
	if removed.Code != http.StatusSeeOther {
		t.Fatalf("web dismiss = %d %q", removed.Code, removed.Body.String())
	}

	history := apiCall(t, handler, token, http.MethodGet, "/api/v1/history", nil)
	assertAPIBody(t, history, http.StatusOK, "Arrival", `"seconds":61`)
	home = apiCall(t, handler, token, http.MethodGet, "/", nil)
	if strings.Contains(home.Body.String(), "Continue watching") {
		t.Fatalf("dismissed item remains in Continue Watching: %q", home.Body.String())
	}
	historyPage := apiCall(t, handler, token, http.MethodGet, "/?view=history", nil)
	assertAPIBody(t, historyPage, http.StatusOK, "Playback history", "Arrival")
}

func signInTestProfile(t *testing.T, handler http.Handler, path, body string) *http.Cookie {
	t.Helper()
	if path == "/setup" && !strings.Contains(body, "totp=") {
		body += "&totp=true"
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(response.Result().Cookies()) != 1 {
		t.Fatalf("sign in = %d, cookies = %v", response.Code, response.Result().Cookies())
	}
	cookie := response.Result().Cookies()[0]
	if path == "/setup" {
		confirmTestFactor(t, handler, cookie, response)
	} else if response.Header().Get("Location") == "/account?mfa=required" {
		confirmTestFactor(t, handler, cookie, requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", cookie))
	}
	return cookie
}

func confirmTestFactor(t *testing.T, handler http.Handler, cookie *http.Cookie, enrollment *httptest.ResponseRecorder) {
	t.Helper()
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(enrollment.Body.String())
	if len(secret) != 2 {
		t.Fatalf("setup factor = %d %q", enrollment.Code, enrollment.Body.String())
	}
	confirmed := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/enable", "code="+testTOTP(t, secret[1], time.Now()), cookie)
	if confirmed.Code != http.StatusSeeOther {
		t.Fatalf("confirm setup factor = %d %q", confirmed.Code, confirmed.Body.String())
	}
}
