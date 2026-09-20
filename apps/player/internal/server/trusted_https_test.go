package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var trustedHTTPSTestToken = strings.Repeat("t", 32)

func TestTrustedHTTPSWizardDefaultsToDuckDNSAndSupportsDeSEC(t *testing.T) { //nolint:cyclop,gocognit // One onboarding scenario covers both provider contracts and persistence.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Trusted HTTPS", "DuckDNS · Easiest", "deSEC · More privacy", `name="provider"`, `value="duckdns" checked`, `placeholder="myhome"`, `placeholder="192.168.1.10 or server.nox"`, "Enter a private IPv4 address or a local hostname", "Example pair:", "myhome.duckdns.org", "myhome-subtitles.duckdns.org", "no router port forwarding is needed", "default <code>38127</code>", "Public remote access is a separate feature", "TCP <code>443</code>", "You need:", "private LAN IPv4 address", "outbound internet access", "temporary DNS TXT record", "without connecting through your router", "renews the certificate automatically", "serves the certificate on this app's LAN port", "password and MFA", "No certificate install", "Test updates this hostname to the LAN address above", "Test DNS connection", `data-trusted-https-test-status`, `/static/theme.js?v=electric-1`, `/static/app.css?v=electric-23`)
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/theme.js", nil))
	assertAPIBody(t, script, http.StatusOK, `request("/api/v1/settings/trusted-https/validate"`, `request("/api/v1/settings/trusted-https/test"`, `request("/api/v1/settings/trusted-https", "PUT")`, "Checking these details", "Nothing was saved")

	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"provider": "desec", "domain": "Family.dedyn.io", "token": trustedHTTPSTestToken, "address": "server.nox", "termsAccepted": true})
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

func TestOwnerCanTestTrustedHTTPSWithoutSaving(t *testing.T) { //nolint:cyclop,gocognit // One API scenario proves success, validation, failure, redaction, and no persistence.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	var checked []trustedhttps.Config
	var checkErr error
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured, TrustedHTTPSCheck: func(_ context.Context, config trustedhttps.Config) error {
		checked = append(checked, config)
		return checkErr
	}})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	input := map[string]any{"provider": "duckdns", "domain": "Family-Media.duckdns.org", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true}
	validated := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/validate", input)
	if validated.Code != http.StatusOK || !strings.Contains(validated.Body.String(), `"status":"valid"`) || strings.Contains(validated.Body.String(), trustedHTTPSTestToken) || len(checked) != 0 {
		t.Fatalf("validation = %d %q, checks = %#v", validated.Code, validated.Body.String(), checked)
	}
	response := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/test", input)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"passed"`) || !strings.Contains(response.Body.String(), `"hostname":"family-media.duckdns.org"`) || strings.Contains(response.Body.String(), trustedHTTPSTestToken) || len(checked) != 1 || checked[0].Domain != "family-media" || checked[0].Token != trustedHTTPSTestToken {
		t.Fatalf("connection test = %d %q, checks = %#v", response.Code, response.Body.String(), checked)
	}
	settings := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil)
	if strings.Contains(settings.Body.String(), `"configured":true`) {
		t.Fatalf("connection test saved settings: %q", settings.Body.String())
	}

	for name, invalid := range map[string]map[string]any{
		"missing token":  {"provider": "duckdns", "domain": "family", "address": "192.168.1.10", "termsAccepted": true},
		"unknown field":  {"provider": "duckdns", "domain": "family", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true, "extra": true},
		"unsafe address": {"provider": "duckdns", "domain": "family", "token": trustedHTTPSTestToken, "address": "203.0.113.10", "termsAccepted": true},
		"missing terms":  {"provider": "duckdns", "domain": "family", "token": trustedHTTPSTestToken, "address": "192.168.1.10"},
	} {
		t.Run(name, func(t *testing.T) {
			validation := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/validate", invalid)
			if validation.Code != http.StatusBadRequest || len(checked) != 1 {
				t.Fatalf("invalid validation = %d %q, checks = %d", validation.Code, validation.Body.String(), len(checked))
			}
			rejected := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/test", invalid)
			if rejected.Code != http.StatusBadRequest || len(checked) != 1 {
				t.Fatalf("invalid test = %d %q, checks = %d", rejected.Code, rejected.Body.String(), len(checked))
			}
		})
	}

	checkErr = errors.New("DuckDNS rejected the request")
	failed := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/test", input)
	if failed.Code != http.StatusBadGateway || !strings.Contains(failed.Body.String(), "connection test failed") || strings.Contains(failed.Body.String(), trustedHTTPSTestToken) || len(checked) != 2 {
		t.Fatalf("failed test = %d %q, checks = %d", failed.Code, failed.Body.String(), len(checked))
	}
}

func TestTrustedHTTPSShowsDeploymentRequirementsBeforeSecrets(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(name string) (string, bool) {
		return "https://server.nox:38127", name == "KINOSAIL_AUTH_URL"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Before entering a token", "configured sign-in address", "https://server.nox:38127", "Sign-in address")
	if body := page.Body.String(); strings.Index(body, "Before entering a token") > strings.Index(body, "Provider token") {
		t.Fatal("deployment requirements were rendered after the provider token")
	}
	conflict := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/validate", map[string]any{"provider": "duckdns", "domain": "family", "token": trustedHTTPSTestToken, "address": "192.168.1.10", "termsAccepted": true})
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "configured sign-in address does not match") || strings.Contains(conflict.Body.String(), trustedHTTPSTestToken) {
		t.Fatalf("deployment validation = %d %q", conflict.Code, conflict.Body.String())
	}
}

func TestOwnerCanConfigureTrustedHTTPSDuringOnboarding(t *testing.T) { //nolint:cyclop,gocognit // One onboarding scenario covers rendered setup, localization, save, and redaction.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	assertAPIBody(t, page, http.StatusOK, "Secure local access", "On by default.", "Jellyfin apps", "Trusted HTTPS is required.", "Trusted HTTPS for phones, TVs, and Jellyfin apps", "Required for Jellyfin apps. Recommended for phones and TVs.", "does not open a router port", "protected secrets file", "add your passkeys again", `id="trusted-https-configuration" open`, "Set up trusted HTTPS", `action="/onboarding/trusted-https"`)
	arabicRequest := requestWithCookieRequest(t, http.MethodGet, "/onboarding/connection", "", owner)
	arabicRequest.Header.Set("Accept-Language", "ar")
	arabic := httptest.NewRecorder()
	handler.ServeHTTP(arabic, arabicRequest)
	if !strings.Contains(arabic.Body.String(), "إعداد HTTPS الموثوق به") || strings.Contains(arabic.Body.String(), "Set up trusted HTTPS") {
		t.Fatalf("Arabic trusted HTTPS setup label = %q", arabic.Body.String())
	}
	saved := requestWithCookie(t, handler, http.MethodPost, "/onboarding/trusted-https", "domain=family-media&token="+trustedHTTPSTestToken+"&address=192.168.1.10&termsAccepted=true", owner)
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/onboarding/connection" {
		t.Fatalf("onboarding save = %d, location = %q", saved.Code, saved.Header().Get("Location"))
	}
	page = requestWithCookie(t, handler, http.MethodGet, "/onboarding/connection", "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Saved for") || !strings.Contains(page.Body.String(), "family-media.duckdns.org") || !strings.Contains(page.Body.String(), "Review trusted HTTPS setup") || strings.Contains(page.Body.String(), trustedHTTPSTestToken) {
		t.Fatalf("saved onboarding = %d %q", page.Code, page.Body.String())
	}
	arabicRequest = requestWithCookieRequest(t, http.MethodGet, "/onboarding/connection", "", owner)
	arabicRequest.Header.Set("Accept-Language", "ar")
	arabic = httptest.NewRecorder()
	handler.ServeHTTP(arabic, arabicRequest)
	if !strings.Contains(arabic.Body.String(), "مراجعة إعداد HTTPS الموثوق به") || strings.Contains(arabic.Body.String(), "Review trusted HTTPS setup") {
		t.Fatalf("Arabic trusted HTTPS review label = %q", arabic.Body.String())
	}
}

func TestOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb(t *testing.T) {
	servertest.AssertOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb(t, trustedHTTPSFixture())
}

func TestTrustedHTTPSRejectsInvalidInputWithoutSideEffects(t *testing.T) {
	servertest.AssertTrustedHTTPSRejectsInvalidInputWithoutSideEffects(t, trustedHTTPSFixture())
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
	testAPI := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/settings/trusted-https/test", map[string]any{"provider": "duckdns", "domain": "other", "token": trustedHTTPSTestToken, "address": "192.168.1.11", "termsAccepted": true})
	disableAPI := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/settings/trusted-https", nil)
	disableWeb := requestWithCookie(t, handler, http.MethodPost, "/settings/trusted-https/disable", "", owner)
	if page.Code != http.StatusOK || onboarding.Code != http.StatusOK || update.Code != http.StatusConflict || testAPI.Code != http.StatusConflict || disableAPI.Code != http.StatusConflict || disableWeb.Code != http.StatusConflict || strings.Contains(page.Body.String()+onboarding.Body.String(), trustedHTTPSTestToken) || !strings.Contains(page.Body.String(), `Configured via Docker: <code>KINOSAIL_DUCKDNS_HTTPS</code>.`) || !strings.Contains(page.Body.String(), `action="/settings/trusted-https" method="post"`) || !strings.Contains(page.Body.String(), `<fieldset disabled aria-disabled="true">`) || !strings.Contains(onboarding.Body.String(), `<fieldset disabled aria-disabled="true">`) {
		t.Fatalf("page=%d onboarding=%d update=%d %q test=%d disable API=%d web=%d", page.Code, onboarding.Code, update.Code, update.Body.String(), testAPI.Code, disableAPI.Code, disableWeb.Code)
	}
}

func TestTrustedHTTPSStorageFailureIsGeneric(t *testing.T) {
	servertest.AssertTrustedHTTPSStorageFailureIsGeneric(t, trustedHTTPSFixture())
}
