package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var trustedHTTPSTestToken = strings.Repeat("t", 32)

func TestTrustedHTTPSWizardDefaultsToDuckDNSAndSupportsDeSEC(t *testing.T) { //nolint:cyclop // The wizard test covers both provider branches.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Trusted HTTPS", "DuckDNS · Easiest", "deSEC · More privacy", `name="provider"`, `value="duckdns" checked`, `placeholder="myhome-subtitles"`, "Example pair:", "myhome.duckdns.org", "myhome-subtitles.duckdns.org", "no router port forwarding is needed", "default <code>38128</code>", "does not make Kinosail Subtitles available away from home", "You need:", "private LAN IPv4 address", "outbound internet access", "temporary DNS TXT record", "without connecting through your router", "renews the certificate automatically", "serves the certificate on this app's LAN port", "password and MFA", "No certificate install")

	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"provider": "desec", "domain": "Family.dedyn.io", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true})
	if saved.Code != http.StatusAccepted || !strings.Contains(saved.Body.String(), `"provider":"desec"`) || !strings.Contains(saved.Body.String(), `"hostname":"family.dedyn.io"`) || strings.Contains(saved.Body.String(), trustedHTTPSTestToken) {
		t.Fatalf("deSEC save = %d %q", saved.Code, saved.Body.String())
	}

	for _, input := range []map[string]any{
		{"provider": "duckdns", "domain": "family-media", "token": "", "address": "192.168.1.10", "termsAccepted": true},
		{"provider": "dynv6", "domain": "family.example", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true},
	} {
		rejected := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", input)
		if rejected.Code != http.StatusBadRequest {
			t.Fatalf("provider switch = %d %q", rejected.Code, rejected.Body.String())
		}
	}

	legacy := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family-media", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true})
	if legacy.Code != http.StatusAccepted || !strings.Contains(legacy.Body.String(), `"provider":"duckdns"`) || !strings.Contains(legacy.Body.String(), `"hostname":"family-media.duckdns.org"`) {
		t.Fatalf("legacy save = %d %q", legacy.Code, legacy.Body.String())
	}
}

func TestOwnerCanConfigureTrustedHTTPSDuringOnboarding(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Secure local access", "On by default.", "Jellyfin apps", "Trusted HTTPS is required.", "Trusted HTTPS (Required for Jellyfin apps)", "Required for Jellyfin apps. Recommended for phones and TVs.", "Get the provider details", "Create a narrow token", "does not open a router port", "protected secrets file", "add your passkeys again", `action="/onboarding/trusted-https"`)
	saved := requestWithCookie(t, handler, http.MethodPost, "/onboarding/trusted-https", "domain=family-media&token="+trustedHTTPSTestToken+"&address=192.168.1.10&termsAccepted=true", owner)
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/onboarding/connection" {
		t.Fatalf("onboarding save = %d, location = %q", saved.Code, saved.Header().Get("Location"))
	}
	page = requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Saved for") || !strings.Contains(page.Body.String(), "family-media.duckdns.org") || strings.Contains(page.Body.String(), trustedHTTPSTestToken) {
		t.Fatalf("saved onboarding = %d %q", page.Code, page.Body.String())
	}
}

func TestOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb(t *testing.T) {
	servertest.AssertOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb(t, trustedHTTPSFixture())
}

func TestTrustedHTTPSRejectsInvalidInputWithoutSideEffects(t *testing.T) {
	servertest.AssertTrustedHTTPSRejectsInvalidInputWithoutSideEffects(t, trustedHTTPSFixture())
}

func TestTrustedHTTPSRejectsConflictingAccessWithoutWriting(t *testing.T) {
	t.Parallel()
	for name, setting := range map[string]string{"tls disabled": "tls.enabled", "public HTTPS remote": "remote.mode"} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
			if err != nil {
				t.Fatal(err)
			}
			value := "false"
			if setting == "remote.mode" {
				value = "https"
			}
			configured.UpdateGUI(setting, value, false)
			handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
			owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
			response := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true})
			if response.Code != http.StatusConflict {
				t.Fatalf("conflicting access = %d %q", response.Code, response.Body.String())
			}
			if contents, readErr := os.ReadFile(filepath.Join(directory, "secrets.json")); readErr == nil && strings.Contains(string(contents), "tls.duckdns") {
				t.Fatalf("conflicting access changed secrets: %q", contents)
			}
		})
	}
}

func TestExternallyManagedTrustedHTTPSIsReadOnlyAndRedacted(t *testing.T) { //nolint:cyclop // One settings flow verifies all read-only and secret-redaction boundaries.
	t.Parallel()
	directory := t.TempDir()
	raw := `{"domain":"family","token":"` + trustedHTTPSTestToken + `","address":"192.168.1.10","termsAccepted":true}`
	configured, err := configuration.Load(directory, "", func(key string) (string, bool) { return raw, key == "KINOSAIL_DUCKDNS_HTTPS" })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	onboarding := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	update := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "other", "token": trustedHTTPSTestToken, "address": "192.168.1.11", "termsAccepted": true})
	disableAPI := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/settings/trusted-https", nil)
	disableWeb := requestWithCookie(t, handler, http.MethodPost, "/settings/trusted-https/disable", "", owner)
	if page.Code != http.StatusOK || onboarding.Code != http.StatusOK || update.Code != http.StatusConflict || disableAPI.Code != http.StatusConflict || disableWeb.Code != http.StatusConflict || strings.Contains(page.Body.String()+onboarding.Body.String(), trustedHTTPSTestToken) || !strings.Contains(page.Body.String(), `Configured via Docker: <code>KINOSAIL_DUCKDNS_HTTPS</code>.`) || !strings.Contains(page.Body.String(), `action="/settings/trusted-https" method="post"`) || !strings.Contains(page.Body.String(), `<fieldset disabled aria-disabled="true">`) || !strings.Contains(onboarding.Body.String(), `<fieldset disabled aria-disabled="true">`) {
		t.Fatalf("page=%d onboarding=%d update=%d %q disable API=%d web=%d", page.Code, onboarding.Code, update.Code, update.Body.String(), disableAPI.Code, disableWeb.Code)
	}
}

func TestTrustedHTTPSStorageFailureIsGeneric(t *testing.T) {
	servertest.AssertTrustedHTTPSStorageFailureIsGeneric(t, trustedHTTPSFixture())
}
