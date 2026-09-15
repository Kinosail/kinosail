package servertest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// APITestCall describes one authenticated API request and its expected status.
type APITestCall struct {
	Method, Path string
	Body         any
	Status       int
}

// AssertAPICalls checks each request's expected status against the real handler.
func AssertAPICalls(t *testing.T, handler http.Handler, token string, calls []APITestCall) {
	t.Helper()
	for _, call := range calls {
		response := APICall(t, handler, token, call.Method, call.Path, call.Body)
		if response.Code != call.Status {
			t.Fatalf("%s %s = %d %q", call.Method, call.Path, response.Code, response.Body.String())
		}
	}
}

// AssertAPIBody checks the response status and required body fragments.
func AssertAPIBody(t *testing.T, response *httptest.ResponseRecorder, status int, values ...string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("API = %d %q", response.Code, response.Body.String())
	}
	for _, value := range values {
		if !bytes.Contains(response.Body.Bytes(), []byte(value)) {
			t.Fatalf("API body lacks %q: %q", value, response.Body.String())
		}
	}
}

// APIFixture binds the real app constructor and its factor generator.
type APIFixture struct {
	NewHandler func(string, string) http.Handler
	TOTP       func(*testing.T, string, time.Time) string
}

// Server provides a populated real app with an authenticated Owner.
func (fixture APIFixture) Server(t *testing.T) (http.Handler, string) {
	t.Helper()
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Arrival.nfo"), []byte("<movie><title>Arrival</title><year>2016</year><tagline>First contact</tagline><director>Denis Villeneuve</director><studio>Paramount</studio></movie>"), 0o600); err != nil {
		t.Fatal(err)
	}
	season := filepath.Join(media, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Severance.S01E01.Good.News.mkv", "Severance.S01E02.Half.Loop.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	music := filepath.Join(media, "Boards of Canada", "Signal")
	if err := os.MkdirAll(music, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(music, "01 Reach for the Dead.mp3"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(music, "01 Reach for the Dead.nfo"), []byte("<song><title>Reach for the Dead</title><artist>Boards of Canada</artist><album>Signal</album><track>1</track></song>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, data)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "API test", "totp": true})
	var session struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	MustJSON(t, setup, &session)
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	return handler, session.Token
}

// WebFormCall submits an authenticated URL-encoded form to the real handler.
func WebFormCall(t *testing.T, handler http.Handler, token, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// MustJSON decodes a response, retaining its status and body in failure reports.
func MustJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("JSON = %d %q: %v", response.Code, response.Body.String(), err)
	}
}
