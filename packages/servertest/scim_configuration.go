package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// SCIMConfiguration binds configuration persistence and real authenticated app handlers.
type SCIMConfiguration[S interface{ String(string) string }] struct {
	AuthURL, Token string
	Load           func(string, string, func(string) (string, bool)) (S, error)
	NewHandler     func(string, S, string, time.Time) http.Handler
	SignIn         func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall        func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
}

// OwnerCanConfigureSCIMPairThroughAPIAndWeb checks paired API and web persistence.
func (fixture SCIMConfiguration[S]) OwnerCanConfigureSCIMPairThroughAPIAndWeb(t *testing.T) {
	t.Helper()
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(key string) (string, bool) {
		return fixture.AuthURL, key == "KINOSAIL_AUTH_URL"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(directory, configured, "", time.Time{})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token := strings.Repeat("s", 32)
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	fixture.assertInitialSCIMPage(t, handler, owner)
	legacyToken := fixture.configureSCIMAPI(t, handler, owner.Value, directory, token, expires)
	newExpiresOn := time.Now().UTC().AddDate(0, 0, 2).Format(time.DateOnly)
	newExpires := newExpiresOn + "T23:59:59Z"
	web := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.scim"}, "token": {""}, "tokenExpiresOn": {newExpiresOn}})
	if web.Code != http.StatusSeeOther {
		t.Fatalf("SCIM web configuration = %d %q", web.Code, web.Body.String())
	}
	fixture.assertSCIMPair(t, directory, legacyToken, newExpires, "saved SCIM web pair")
	fixture.assertInvalidSCIMConfiguration(t, handler, owner.Value, directory, token, expires, newExpiresOn, legacyToken, newExpires)
	fixture.assertConfiguredSCIMPage(t, handler, owner)
	if removed := APICall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.scim", nil); removed.Code != http.StatusAccepted {
		t.Fatalf("SCIM API reset = %d %q", removed.Code, removed.Body.String())
	}
	fixture.assertSCIMPair(t, directory, "", "", "reset SCIM pair")
}

func (fixture SCIMConfiguration[S]) assertInvalidSCIMConfiguration(t *testing.T, handler http.Handler, session, directory, token, expires, expiresOn, savedToken, savedExpiration string) {
	t.Helper()
	unexpectedAPI := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/server.name", map[string]string{"value": "Changed", "tokenExpiresAt": savedExpiration})
	unknownAPI := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.scim", map[string]string{"token": token, "tokenExpiresAt": expires, "unknown": "true"})
	oversizedAPI := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.scim", map[string]string{"token": strings.Repeat("x", 257), "tokenExpiresAt": expires})
	ambiguousWeb := WebFormCall(t, handler, session, "/settings/configuration", map[string][]string{"key": {"integrations.scim"}, "token": {token, strings.Repeat("x", 32)}, "tokenExpiresOn": {expiresOn}})
	unknownWeb := WebFormCall(t, handler, session, "/settings/configuration", map[string][]string{"key": {"integrations.scim"}, "token": {token}, "tokenExpiresOn": {expiresOn}, "unknown": {"true"}})
	loaded, err := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if unexpectedAPI.Code != http.StatusBadRequest || unknownAPI.Code != http.StatusBadRequest || oversizedAPI.Code != http.StatusConflict || ambiguousWeb.Code != http.StatusBadRequest || unknownWeb.Code != http.StatusBadRequest || err != nil || loaded.String("server.name") != "" || loaded.String("integrations.scim.token") != savedToken || loaded.String("integrations.scim.token_expires_at") != savedExpiration {
		t.Fatalf("invalid configuration inputs API=%d unknown=%d oversized=%d ambiguous=%d webUnknown=%d err=%v", unexpectedAPI.Code, unknownAPI.Code, oversizedAPI.Code, ambiguousWeb.Code, unknownWeb.Code, err)
	}
}

// ExpiredSCIMConfigurationKeepsOwnerRecoveryAvailable checks expired-token isolation.
func (fixture SCIMConfiguration[S]) ExpiredSCIMConfigurationKeepsOwnerRecoveryAvailable(t *testing.T) {
	t.Helper()
	t.Parallel()
	directory := t.TempDir()
	expires := time.Now().UTC().Add(-time.Hour)
	configured, err := fixture.Load(directory, "", func(key string) (string, bool) {
		values := map[string]string{"KINOSAIL_AUTH_URL": "https://media.example", "KINOSAIL_SCIM_TOKEN": fixture.Token, "KINOSAIL_SCIM_TOKEN_EXPIRES_AT": expires.Format(time.RFC3339)}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(directory, configured, fixture.Token, expires)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	page := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	denied := SCIMCall(t, handler, fixture.Token, http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "The configured token expired") || strings.Contains(page.Body.String(), fixture.Token) || denied.Code != http.StatusUnauthorized {
		t.Fatalf("expired SCIM recovery page=%d leaked=%t SCIM=%d", page.Code, strings.Contains(page.Body.String(), fixture.Token), denied.Code)
	}
}

func (fixture SCIMConfiguration[S]) assertInitialSCIMPage(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	initialPage := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if initialPage.Code != http.StatusOK || !strings.Contains(initialPage.Body.String(), fixture.AuthURL+"/scim/v2") || !regexp.MustCompile(`name="token" type="password" value="[A-Za-z0-9_-]{43}"`).MatchString(initialPage.Body.String()) {
		t.Fatalf("initial SCIM setup = %d", initialPage.Code)
	}
}

func (fixture SCIMConfiguration[S]) assertConfiguredSCIMPage(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	page := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="integrations.scim"`) || !strings.Contains(page.Body.String(), fixture.AuthURL+"/scim/v2") || !strings.Contains(page.Body.String(), `name="tokenExpiresOn" type="date"`) || !strings.Contains(page.Body.String(), `Leave blank to keep the configured token`) {
		t.Fatalf("SCIM configuration page = %d %q", page.Code, page.Body.String())
	}
}

func (fixture SCIMConfiguration[S]) configureSCIMAPI(t *testing.T, handler http.Handler, session, directory, token, expires string) string {
	t.Helper()
	if invalid := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.scim", map[string]string{"token": token}); invalid.Code != http.StatusConflict {
		t.Fatalf("incomplete SCIM API configuration = %d %q", invalid.Code, invalid.Body.String())
	}
	for _, name := range []string{"configuration.json", "secrets.json"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("incomplete SCIM API configuration left %s: %v", name, err)
		}
	}
	saved := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.scim", map[string]string{"token": token, "tokenExpiresAt": expires})
	if saved.Code != http.StatusAccepted {
		t.Fatalf("SCIM API configuration = %d %q", saved.Code, saved.Body.String())
	}
	fixture.assertSCIMPair(t, directory, token, expires, "saved SCIM API pair")
	legacyToken, legacyExpires := strings.Repeat("x", 32), time.Now().UTC().Add(2*time.Hour).Format(time.RFC3339)
	legacy := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.scim.token", map[string]string{"value": legacyToken, "tokenExpiresAt": legacyExpires})
	if legacy.Code != http.StatusAccepted {
		t.Fatalf("legacy SCIM API configuration = %d %q", legacy.Code, legacy.Body.String())
	}

	return legacyToken
}

func (fixture SCIMConfiguration[S]) assertSCIMPair(t *testing.T, directory, token, expires, label string) {
	t.Helper()
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if loadErr != nil || loaded.String("integrations.scim.token") != token || loaded.String("integrations.scim.token_expires_at") != expires {
		t.Fatalf("%s = %q %q err=%v", label, loaded.String("integrations.scim.token"), loaded.String("integrations.scim.token_expires_at"), loadErr)
	}
}
