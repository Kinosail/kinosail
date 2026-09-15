package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerConfiguresOpenSubtitlesAtomicallyThroughWebAndAPI(t *testing.T) { //nolint:cyclop,funlen // One end-to-end test covers the shared atomic operation and both adapters.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: directory, CacheDir: t.TempDir(), RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	for _, expected := range []string{`id="integrations.opensubtitles"`, `name="apiKey"`, `name="username"`, `name="password"`, "Save all three values together"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("configuration page lacks %q", expected)
		}
	}
	if strings.Contains(page.Body.String(), `id="integrations.opensubtitles.api_key"`) {
		t.Fatal("configuration page exposes separate OpenSubtitles forms")
	}

	missing := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.opensubtitles"}, "username": {"owner"}, "password": {"password"}})
	unknown := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.opensubtitles"}, "apiKey": {"app-key"}, "username": {"owner"}, "password": {"password"}, "unknown": {"x"}})
	oversized := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.opensubtitles"}, "apiKey": {strings.Repeat("x", 4097)}, "username": {"owner"}, "password": {"password"}})
	before, beforeErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if missing.Code != http.StatusBadRequest || unknown.Code != http.StatusBadRequest || oversized.Code != http.StatusBadRequest || beforeErr != nil || before.String("integrations.opensubtitles.api_key") != "" {
		t.Fatalf("rejected form changed state: missing=%d unknown=%d oversized=%d key=%q error=%v", missing.Code, unknown.Code, oversized.Code, before.String("integrations.opensubtitles.api_key"), beforeErr)
	}

	saved := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.opensubtitles"}, "apiKey": {"first-key"}, "username": {"owner"}, "password": {"first-password"}})
	reloaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	updatedPage := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if saved.Code != http.StatusSeeOther || loadErr != nil || reloaded.String("integrations.opensubtitles.api_key") != "first-key" || reloaded.String("integrations.opensubtitles.username") != "owner" || reloaded.String("integrations.opensubtitles.password") != "first-password" || !strings.Contains(updatedPage.Body.String(), "OpenSubtitles credentials are configured.") || strings.Contains(updatedPage.Body.String(), "first-password") {
		t.Fatalf("web save failed: saved=%d key=%q username=%q error=%v", saved.Code, reloaded.String("integrations.opensubtitles.api_key"), reloaded.String("integrations.opensubtitles.username"), loadErr)
	}

	changed := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.opensubtitles", map[string]any{"apiKey": "second-key", "username": "", "password": ""})
	after, afterErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if changed.Code != http.StatusAccepted || afterErr != nil || after.String("integrations.opensubtitles.api_key") != "second-key" || after.String("integrations.opensubtitles.username") != "owner" || after.String("integrations.opensubtitles.password") != "first-password" {
		t.Fatalf("API update failed: changed=%d key=%q username=%q error=%v", changed.Code, after.String("integrations.opensubtitles.api_key"), after.String("integrations.opensubtitles.username"), afterErr)
	}

	removed := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.opensubtitles", nil)
	final, finalErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if removed.Code != http.StatusAccepted || finalErr != nil || final.String("integrations.opensubtitles.api_key") != "" || final.String("integrations.opensubtitles.username") != "" || final.String("integrations.opensubtitles.password") != "" {
		t.Fatalf("API reset failed: removed=%d error=%v", removed.Code, finalErr)
	}
}
