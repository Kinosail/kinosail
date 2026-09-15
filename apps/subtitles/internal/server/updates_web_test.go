package server_test

import (
	"net/http"
	"net/url"
	"testing"
)

func TestOwnerUpdatePreferenceHasWebAndAPIParity(t *testing.T) {
	handler, token := apiServer(t)
	settings := apiCall(t, handler, token, http.MethodGet, "/settings", nil)
	assertAPIBody(t, settings, http.StatusOK, `id="updates"`, "Software updates", "Choose when to update", "Install updates automatically", "public IP address", "does not send its version", "Check and update now", `action="/settings/updates/check"`)
	onboarding := apiCall(t, handler, token, http.MethodGet, "/onboarding/connection", nil)
	assertAPIBody(t, onboarding, http.StatusOK, "Choose how Kinosail updates", "Choose when to update", "Install updates automatically", "Check and update now", `action="/onboarding/updates"`, `action="/onboarding/updates/check"`)

	invalid := webFormCall(t, handler, token, "/settings/updates", url.Values{"mode": {"automatic"}, "unexpected": {"true"}})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid web preference = %d %q", invalid.Code, invalid.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/updates", nil), http.StatusOK, `"automatic":true`, `"state":"not-checked"`)

	saved := webFormCall(t, handler, token, "/onboarding/updates", url.Values{"mode": {"automatic"}})
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/onboarding/connection#updates" {
		t.Fatalf("web preference = %d location=%q", saved.Code, saved.Header().Get("Location"))
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/updates", nil), http.StatusOK, `"automatic":true`, `"state":"not-checked"`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/updates", map[string]any{"automatic": false}), http.StatusOK, `"automatic":false`)
	settings = apiCall(t, handler, token, http.MethodGet, "/settings", nil)
	assertAPIBody(t, settings, http.StatusOK, `name="mode" value="manual" checked`, "Choose when to update")
}
