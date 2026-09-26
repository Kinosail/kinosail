package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubDLKeyChangesTakeEffectWithoutRestart(t *testing.T) {
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/me" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("api_key") == "good-key" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
			return
		}
		http.Error(writer, "rejected", http.StatusUnauthorized)
	}))
	t.Cleanup(remote.Close)
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: directory, CacheDir: t.TempDir(), RequireAuth: true, Configuration: configured, Subtitles: server.SubtitleConfig{URL: remote.URL + "/api/v1"}})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	check := func(wantAttempted, wantConnected int) {
		t.Helper()
		result := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/subtitle-providers/test", map[string]any{})
		var body struct{ Attempted, Connected int }
		if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil || result.Code != http.StatusOK || body.Attempted != wantAttempted || body.Connected != wantConnected {
			t.Fatalf("credential check = %d %s; want %d attempted, %d connected: %v", result.Code, result.Body.String(), wantAttempted, wantConnected, err)
		}
	}
	check(0, 0)
	bad := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subdl.api_key", map[string]any{"value": "bad-key"})
	if bad.Code != http.StatusAccepted || !strings.Contains(bad.Body.String(), `"restartRequired":false`) {
		t.Fatalf("save response = %d %s", bad.Code, bad.Body.String())
	}
	check(1, 0)
	good := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subdl.api_key", map[string]any{"value": "good-key"})
	if good.Code != http.StatusAccepted {
		t.Fatalf("replacement response = %d %s", good.Code, good.Body.String())
	}
	check(1, 1)
	invalid := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subdl.api_key", map[string]any{"value": "invalid\nkey"})
	if invalid.Code != http.StatusConflict {
		t.Fatalf("invalid replacement = %d %s", invalid.Code, invalid.Body.String())
	}
	check(1, 1)
	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Changes take effect immediately.") {
		t.Fatalf("SubDL settings copy = %d", page.Code)
	}
	fields := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/configuration", nil)
	var values struct {
		Settings []struct {
			Key             string `json:"key"`
			RestartRequired bool   `json:"restartRequired"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(fields.Body.Bytes(), &values); err != nil || fields.Code != http.StatusOK {
		t.Fatalf("configuration metadata = %d: %v", fields.Code, err)
	}
	found := false
	for _, field := range values.Settings {
		if field.Key == "integrations.subdl.api_key" {
			found = true
			if field.RestartRequired {
				t.Fatal("SubDL API key still claims a restart is required")
			}
		}
	}
	if !found {
		t.Fatal("SubDL API key is missing from configuration metadata")
	}
	reset := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.subdl.api_key", nil)
	if reset.Code != http.StatusAccepted || !strings.Contains(reset.Body.String(), `"restartRequired":false`) {
		t.Fatalf("reset response = %d %s", reset.Code, reset.Body.String())
	}
	check(0, 0)
}

func TestOpenSubtitlesAndSubSourceKeysChangeWithoutRestart(t *testing.T) {
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/login":
			if request.Header.Get("Api-Key") != "open-good" {
				http.Error(writer, "rejected", http.StatusUnauthorized)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"token":"session","status":200}`))
		case "/api/v1/languages":
			if request.Header.Get("X-API-Key") != "source-good" {
				http.Error(writer, "rejected", http.StatusUnauthorized)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(remote.Close)
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: directory, CacheDir: t.TempDir(), RequireAuth: true, Configuration: configured, Subtitles: server.SubtitleConfig{OpenSubtitles: server.OpenSubtitlesConfig{URL: remote.URL + "/api/v1"}, SubSource: server.SubSourceConfig{URL: remote.URL + "/api/v1"}}})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	check := func(wantAttempted, wantConnected int) {
		t.Helper()
		result := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/subtitle-providers/test", map[string]any{})
		var body struct{ Attempted, Connected int }
		if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil || result.Code != http.StatusOK || body.Attempted != wantAttempted || body.Connected != wantConnected {
			t.Fatalf("credential check = %d %s; want %d attempted, %d connected: %v", result.Code, result.Body.String(), wantAttempted, wantConnected, err)
		}
	}
	check(0, 0)
	for _, key := range []string{"open-bad", "open-good"} {
		result := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.opensubtitles", map[string]any{"apiKey": key, "username": "owner", "password": "password"})
		if result.Code != http.StatusAccepted || !strings.Contains(result.Body.String(), `"restartRequired":false`) {
			t.Fatalf("OpenSubtitles save = %d %s", result.Code, result.Body.String())
		}
		if key == "open-bad" {
			check(1, 0)
		} else {
			check(1, 1)
		}
	}
	for _, key := range []string{"source-bad", "source-good"} {
		result := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subsource", map[string]any{"apiKey": key, "personalUse": true})
		if result.Code != http.StatusAccepted || !strings.Contains(result.Body.String(), `"restartRequired":false`) {
			t.Fatalf("SubSource save = %d %s", result.Code, result.Body.String())
		}
		page := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "SubSource</strong> · <span class=\"subtitle-state \">untested</span>") {
			t.Fatalf("SubSource status did not refresh after save: %d", page.Code)
		}
		if key == "source-bad" {
			check(2, 1)
		} else {
			check(2, 2)
		}
	}
	for _, path := range []string{"/api/v1/configuration/integrations.opensubtitles", "/api/v1/configuration/integrations.subsource"} {
		result := apiCall(t, handler, owner.Value, http.MethodDelete, path, nil)
		if result.Code != http.StatusAccepted || !strings.Contains(result.Body.String(), `"restartRequired":false`) {
			t.Fatalf("reset %s = %d %s", path, result.Code, result.Body.String())
		}
	}
	check(0, 0)
	page := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	advanced := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if page.Code != http.StatusOK || advanced.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Changes take effect immediately.") || !strings.Contains(advanced.Body.String(), "Changes take effect immediately.") {
		t.Fatalf("provider settings copy = %d, advanced copy = %d", page.Code, advanced.Code)
	}
}
