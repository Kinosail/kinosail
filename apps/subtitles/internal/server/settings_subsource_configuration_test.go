package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerConfiguresSubSourceAtomicallyThroughWebAndAPI(t *testing.T) { //nolint:cyclop // One end-to-end test covers the shared atomic operation and both adapters.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: t.TempDir(), DataDir: directory, CacheDir: t.TempDir(), RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	missingTerms := webFormCall(t, handler, owner.Value, "/settings/subtitles/subsource", map[string][]string{"apiKey": {"first-key"}})
	unknown := webFormCall(t, handler, owner.Value, "/settings/subtitles/subsource", map[string][]string{"apiKey": {"first-key"}, "personalUse": {"true"}, "unknown": {"x"}})
	before, beforeErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if missingTerms.Code != http.StatusBadRequest || unknown.Code != http.StatusBadRequest || beforeErr != nil || before.String("integrations.subsource.api_key") != "" || before.Bool("integrations.subsource.personal_use") {
		t.Fatalf("rejected form changed state: missing=%d unknown=%d key=%q accepted=%t error=%v", missingTerms.Code, unknown.Code, before.String("integrations.subsource.api_key"), before.Bool("integrations.subsource.personal_use"), beforeErr)
	}

	saved := webFormCall(t, handler, owner.Value, "/settings/subtitles/subsource", map[string][]string{"apiKey": {"first-key"}, "personalUse": {"true"}})
	page := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	reloaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusSeeOther || page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "A SubSource key and personal-use acceptance are saved.") || strings.Contains(page.Body.String(), "first-key") || loadErr != nil || reloaded.String("integrations.subsource.api_key") != "first-key" || !reloaded.Bool("integrations.subsource.personal_use") {
		t.Fatalf("web save failed: saved=%d page=%d key=%q accepted=%t error=%v", saved.Code, page.Code, reloaded.String("integrations.subsource.api_key"), reloaded.Bool("integrations.subsource.personal_use"), loadErr)
	}

	declined := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subsource", map[string]any{"apiKey": "rejected-key", "personalUse": false})
	oversized := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subsource", map[string]any{"apiKey": strings.Repeat("x", 4097), "personalUse": true})
	changed := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.subsource", map[string]any{"apiKey": "second-key", "personalUse": true})
	after, afterErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if declined.Code != http.StatusBadRequest || oversized.Code != http.StatusConflict || changed.Code != http.StatusAccepted || afterErr != nil || after.String("integrations.subsource.api_key") != "second-key" {
		t.Fatalf("API update failed: declined=%d oversized=%d changed=%d key=%q error=%v", declined.Code, oversized.Code, changed.Code, after.String("integrations.subsource.api_key"), afterErr)
	}

	removed := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.subsource", nil)
	final, finalErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if removed.Code != http.StatusAccepted || finalErr != nil || final.String("integrations.subsource.api_key") != "" || final.Bool("integrations.subsource.personal_use") {
		t.Fatalf("API reset failed: removed=%d key=%q accepted=%t error=%v", removed.Code, final.String("integrations.subsource.api_key"), final.Bool("integrations.subsource.personal_use"), finalErr)
	}
}
