package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestInvalidExplicitCredentialsDoNotFallBackToAValidCookie(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	for name, credential := range map[string]struct{ path, header, value string }{
		"Bearer":               {"/api/v1/settings", "Authorization", "Bearer invalid"},
		"ApiKey":               {"/api/v1/settings", "ApiKey", "invalid"},
		"X-Emby-Token":         {"/api/v1/settings", "X-Emby-Token", "invalid"},
		"X-MediaBrowser-Token": {"/api/v1/settings", "X-MediaBrowser-Token", "invalid"},
		"MediaBrowser":         {"/api/v1/settings", "Authorization", `MediaBrowser Token="invalid"`},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, credential.path, nil)
			request.AddCookie(owner)
			if credential.header != "" {
				request.Header.Set(credential.header, credential.value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("invalid explicit credential fell back to cookie: %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestExplicitCredentialsTakePrecedenceOverAnotherProfilesCookie(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	created := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create Viewer = %d %q", created.Code, created.Body.String())
	}
	login := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var viewer struct{ Token string }
	mustJSON(t, login, &viewer)

	for name, header := range map[string]struct{ key, value string }{
		"Bearer":       {"Authorization", "Bearer " + viewer.Token},
		"ApiKey":       {"ApiKey", viewer.Token},
		"X-Emby-Token": {"X-Emby-Token", viewer.Token},
		"MediaBrowser": {"Authorization", `MediaBrowser Token="` + viewer.Token + `"`},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
			request.AddCookie(owner)
			request.Header.Set(header.key, header.value)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"Viewer"`) || strings.Contains(response.Body.String(), `"name":"Owner"`) {
				t.Fatalf("credential precedence = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestQueryCredentialsCannotAuthorizeJellyfinRequests(t *testing.T) {
	t.Parallel()
	handler := newJellyfinServer(t, server.Config{RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	paths := []string{
		"/Sessions/Logout", "/Sessions/Capabilities", "/Items/missing/PlaybackInfo",
		"/UserFavoriteItems/missing", "/Users/missing/PlayedItems/missing", "/QuickConnect/Authorize",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path+"?api_key="+owner.Value, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("query credential authorized mutation = %d %q", response.Code, response.Body.String())
			}
		})
	}
	read := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/Users/Me?api_key="+owner.Value, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, read)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("Jellyfin query credential authorized HEAD = %d %q", response.Code, response.Body.String())
	}
}

func TestPasswordLoginRotatesAnUntrustedBrowserSession(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	disableTestMFA(t, handler, owner.Value)
	addTestViewer(t, handler, owner)
	attacker := &http.Cookie{Name: "__Host-kinosail_session", Value: "attacker-chosen-session", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(attacker)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusSeeOther || len(cookies) != 1 || cookies[0].Value == "" || cookies[0].Value == attacker.Value {
		t.Fatalf("login session was not rotated: %d cookies=%v", response.Code, cookies)
	}

	if stale := requestWithCookie(t, handler, http.MethodGet, "/", "", attacker); stale.Code != http.StatusSeeOther || stale.Header().Get("Location") != "/login" {
		t.Fatalf("attacker session became valid = %d %q", stale.Code, stale.Header().Get("Location"))
	}
	if current := requestWithCookie(t, handler, http.MethodGet, "/", "", cookies[0]); current.Code != http.StatusOK {
		t.Fatalf("rotated session = %d %q", current.Code, current.Body.String())
	}
}

func TestPasswordResetImmediatelyRevokesActiveSessions(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	created := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var viewer struct {
		ID string `json:"id"`
	}
	mustJSON(t, created, &viewer)
	login := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var session struct {
		Token string `json:"token"`
	}
	mustJSON(t, login, &session)

	reset := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/profiles/"+viewer.ID+"/password", map[string]any{"password": "new-viewer-password"})
	if reset.Code != http.StatusNoContent {
		t.Fatalf("password reset = %d %q", reset.Code, reset.Body.String())
	}
	if stale := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/me", nil); stale.Code != http.StatusUnauthorized {
		t.Fatalf("pre-reset session remained valid = %d %q", stale.Code, stale.Body.String())
	}
	if fresh := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "new-viewer-password"}); fresh.Code != http.StatusCreated {
		t.Fatalf("new password login = %d %q", fresh.Code, fresh.Body.String())
	}
}

func TestCredentialRateLimitSpansWebAPIAndJellyfin(t *testing.T) { //nolint:cyclop // Alternating adapters is the behavior under test.
	t.Parallel()
	handler := newJellyfinServer(t, server.Config{RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	addTestViewer(t, handler, owner)
	var last *httptest.ResponseRecorder
	for attempt := range 11 {
		var request *http.Request
		switch attempt % 3 {
		case 0:
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Sam&password=wrong-password"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		case 1:
			body, _ := json.Marshal(map[string]string{"name": "Sam", "password": "wrong-password"})
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
		default:
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Users/AuthenticateByName", strings.NewReader(`{"Username":"Sam","Pw":"wrong-password"}`))
			request.Header.Set("Content-Type", "application/json")
		}
		request.RemoteAddr = "192.0.2.80:1234"
		last = httptest.NewRecorder()
		handler.ServeHTTP(last, request)
		if attempt < 10 && last.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d %q", attempt+1, last.Code, last.Body.String())
		}
	}
	if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") != "60" {
		t.Fatalf("cross-adapter rate limit = %d headers=%v body=%q", last.Code, last.Header(), last.Body.String())
	}
}
