package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JellyfinSettingsFixture binds compatibility contracts to real app configuration and HTTP helpers.
type JellyfinSettingsFixture struct {
	Port                string
	OnboardingFragments []string
	NewHandler          func(*testing.T, string, string, bool) http.Handler
	APIServer           func(*testing.T) (http.Handler, string)
	APICall             func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
	JellyfinCall        func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
	WebFormCall         func(*testing.T, http.Handler, string, string, url.Values) *httptest.ResponseRecorder
}

// JellyfinCompatibilityAPIIsOffByDefaultAndCanBeEnabled runs the shared Jellyfin settings regression contract.
func JellyfinCompatibilityAPIIsOffByDefaultAndCanBeEnabled(t *testing.T, fixture JellyfinSettingsFixture) {
	t.Parallel()

	handler, token := fixture.APIServer(t)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"jellyfinCompatibility":false`)
	for _, path := range []string{
		"/System/Info/Public", "/system/info/public", "/QuickConnect/Enabled", "/Users/Public", "/UserViews", "/UserItems/Resume",
		"/UserPlayedItems/id", "/UserFavoriteItems/id", "/Items", "/Shows/id/Episodes", "/Videos/id/stream", "/Audio/id/stream",
		"/Sessions/Logout", "/Branding/Configuration", "/MediaSegments/id", "/Library/MediaFolders", "/Users", "/Auth/Keys",
	} {
		if response := fixture.JellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusNotFound {
			t.Fatalf("disabled Jellyfin compatibility at %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil), http.StatusOK, `"items"`)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodPut, "/api/v1/settings/jellyfin", map[string]any{"enabled": true}), http.StatusConflict, "trusted HTTPS is required")
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"jellyfinCompatibility":false`)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family-media", "token": strings.Repeat("t", 32), "address": "192.168.1.10", "termsAccepted": true}), http.StatusAccepted, `"restartRequired":true`)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodPut, "/api/v1/settings/jellyfin", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"jellyfinCompatibility":true`, `"jellyfinURL":"https://family-media.duckdns.org"`)
	if response := fixture.JellyfinCall(t, handler, http.MethodGet, "/System/Info/Public", "", ""); response.Code != http.StatusOK {
		t.Fatalf("enabled Jellyfin compatibility = %d %q", response.Code, response.Body.String())
	}
}

// JellyfinCompatibilityWebSettingPersists runs the shared Jellyfin settings regression contract.
func JellyfinCompatibilityWebSettingPersists(t *testing.T, fixture JellyfinSettingsFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	origin := "https://media.example:" + fixture.Port
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"jellyfinId":"legacy"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(t, dataDir, origin, false)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/settings", nil))
	assertDisabledJellyfinGuidance(t, page.Body.String())
	trusted := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/settings/trusted-https", strings.NewReader("domain=family-media&token="+strings.Repeat("t", 32)+"&address=192.168.1.10&termsAccepted=true"))
	trusted.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	trustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(trustedResponse, trusted)
	if trustedResponse.Code != http.StatusSeeOther {
		t.Fatalf("trusted HTTPS save = %d %q", trustedResponse.Code, trustedResponse.Body.String())
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/settings/jellyfin", strings.NewReader("enabled=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	handler = fixture.NewHandler(t, dataDir, origin, true)
	if enabled := fixture.JellyfinCall(t, handler, http.MethodGet, origin+"/System/Info/Public", "", ""); response.Code != http.StatusSeeOther || enabled.Code != http.StatusOK {
		t.Fatalf("save = %d, persisted Jellyfin compatibility = %d %q", response.Code, enabled.Code, enabled.Body.String())
	}
	page = httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/settings", nil))
	if !strings.Contains(page.Body.String(), `name="enabled" value="true" checked`) || !strings.Contains(page.Body.String(), `Connect address: <code>https://family-media.duckdns.org:`+fixture.Port+`</code>`) {
		t.Fatalf("enabled Jellyfin setting or address is missing: %q", page.Body.String())
	}
}

// JellyfinOnboardingRequiresDuckDNSBeforeEnable runs the shared Jellyfin settings regression contract.
func JellyfinOnboardingRequiresDuckDNSBeforeEnable(t *testing.T, fixture JellyfinSettingsFixture) {
	t.Parallel()

	handler, token := fixture.APIServer(t)
	page := fixture.APICall(t, handler, token, http.MethodGet, "/onboarding/connection", nil)
	assertJellyfinSettingsResponse(t, page, http.StatusOK, "Trusted HTTPS", "Required for Jellyfin apps", "Many Jellyfin apps reject private certificates", "Jellyfin routes stay unavailable", `action="/onboarding/jellyfin"`)
	assertContains(t, "onboarding", page.Body.String(), fixture.OnboardingFragments...)
	if strings.Contains(page.Body.String(), `name="enabled" value="true" checked`) {
		t.Fatalf("onboarding enabled Jellyfin by default: %q", page.Body.String())
	}

	rejected := fixture.WebFormCall(t, handler, token, "/onboarding/jellyfin", map[string][]string{"enabled": {"true"}})
	assertJellyfinSettingsResponse(t, rejected, http.StatusConflict, "trusted HTTPS is required")
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"jellyfinCompatibility":false`)
	assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family-media", "token": strings.Repeat("t", 32), "address": "192.168.1.10", "termsAccepted": true}), http.StatusAccepted, `"restartRequired":true`)
	page = fixture.APICall(t, handler, token, http.MethodGet, "/onboarding/connection", nil)
	assertJellyfinSettingsResponse(t, page, http.StatusOK, "You can now enable Jellyfin apps, then restart Kinosail once.")

	saved := fixture.WebFormCall(t, handler, token, "/onboarding/jellyfin", map[string][]string{"enabled": {"true"}})
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/onboarding/connection#jellyfin" {
		t.Fatalf("onboarding save = %d, location = %q", saved.Code, saved.Header().Get("Location"))
	}
	page = fixture.APICall(t, handler, token, http.MethodGet, "/onboarding/connection", nil)
	assertJellyfinSettingsResponse(t, page, http.StatusOK, "Jellyfin routes are available")
	if !strings.Contains(page.Body.String(), `name="enabled" value="true" checked`) {
		t.Fatalf("onboarding Jellyfin choice is not checked: %q", page.Body.String())
	}
	activity := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/activity?limit=100", nil)
	if strings.Count(activity.Body.String(), `"action":"settings.jellyfin.updated"`) < 2 || !strings.Contains(activity.Body.String(), `"result":"failure"`) || !strings.Contains(activity.Body.String(), `"result":"success"`) || !strings.Contains(activity.Body.String(), `"after.enabled":"false"`) {
		t.Fatalf("Jellyfin setup activity lacks rejected and saved states: %s", activity.Body.String())
	}
}

// JellyfinWebSettingRejectsAmbiguousInputWithoutSideEffects runs the shared Jellyfin settings regression contract.
func JellyfinWebSettingRejectsAmbiguousInputWithoutSideEffects(t *testing.T, fixture JellyfinSettingsFixture) {
	t.Parallel()

	handler, token := fixture.APIServer(t)
	for name, form := range map[string]map[string][]string{
		"unknown":  {"enabled": {"true"}, "unexpected": {"true"}},
		"multiple": {"enabled": {"true", "true"}},
		"invalid":  {"enabled": {"yes"}},
	} {
		t.Run(name, func(t *testing.T) {
			response := fixture.WebFormCall(t, handler, token, "/settings/jellyfin", form)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid form = %d %q", response.Code, response.Body.String())
			}
			assertJellyfinSettingsResponse(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"jellyfinCompatibility":false`)
		})
	}
}

func assertDisabledJellyfinGuidance(t *testing.T, markup string) {
	t.Helper()
	if strings.Contains(markup, `name="enabled" value="true" checked`) {
		t.Fatalf("default Jellyfin setting is checked: %q", markup)
	}
	if strings.Contains(markup, `Connect address:`) || !strings.Contains(markup, `Trusted HTTPS is required.`) || !strings.Contains(markup, `fieldset disabled aria-disabled="true"`) {
		t.Fatalf("disabled Jellyfin guidance is wrong: %q", markup)
	}
}

func assertJellyfinSettingsResponse(t *testing.T, response *httptest.ResponseRecorder, status int, fragments ...string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("API = %d %q", response.Code, response.Body.String())
	}
	assertContains(t, "API body", response.Body.String(), fragments...)
}
