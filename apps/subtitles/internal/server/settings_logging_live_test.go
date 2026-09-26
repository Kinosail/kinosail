package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/appcli"
)

func TestLoggingLevelChangesWithoutRestart(t *testing.T) { //nolint:gocognit,cyclop // One Owner journey verifies activation, rejection, metadata, persistence, and reset.
	previous := slog.Default()
	appcli.ConfigureLogging("info")
	t.Cleanup(func() { slog.SetDefault(previous) })
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if slog.Default().Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("debug logging was enabled before the change")
	}
	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/logging.level", map[string]string{"value": "debug"})
	if saved.Code != http.StatusAccepted || !strings.Contains(saved.Body.String(), `"restartRequired":false`) || !slog.Default().Enabled(t.Context(), slog.LevelDebug) {
		t.Fatalf("live logging change = %d %s", saved.Code, saved.Body.String())
	}
	fields := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/configuration", nil)
	var metadata struct {
		Settings []struct {
			Key     string `json:"key"`
			Restart bool   `json:"restartRequired"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(fields.Body.Bytes(), &metadata); err != nil || fields.Code != http.StatusOK {
		t.Fatalf("configuration metadata = %d: %v", fields.Code, err)
	}
	found := false
	for _, field := range metadata.Settings {
		if field.Key == "logging.level" {
			found = true
			if field.Restart {
				t.Fatal("logging level still claims a restart is required")
			}
		}
	}
	if !found {
		t.Fatal("logging level is missing from configuration metadata")
	}
	for _, input := range []map[string]any{{"value": ""}, {"value": "verbose"}, {"value": strings.Repeat("x", 20<<10)}, {"value": 7}, {"value": "error", "unknown": true}} {
		rejected := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/logging.level", input)
		if rejected.Code == http.StatusAccepted || !slog.Default().Enabled(t.Context(), slog.LevelDebug) {
			t.Fatalf("invalid logging input changed active level: %d", rejected.Code)
		}
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("logging.level") != "debug" {
		t.Fatalf("invalid logging values changed stored level: %v, %q", err, loaded.String("logging.level"))
	}
	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="logging.level"`) || !strings.Contains(page.Body.String(), "Changes take effect immediately.") {
		t.Fatalf("logging configuration page = %d", page.Code)
	}
	reset := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/logging.level", nil)
	if reset.Code != http.StatusAccepted || !strings.Contains(reset.Body.String(), `"restartRequired":false`) || slog.Default().Enabled(t.Context(), slog.LevelDebug) {
		t.Fatalf("logging reset = %d %s", reset.Code, reset.Body.String())
	}
}
