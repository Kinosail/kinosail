package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ConfigurationHTTP runs the same persistence, secret, and adapter contracts for both apps.
type ConfigurationHTTP[Source ~string, S interface {
	String(string) string
	Int(string) int
	Bool(string) bool
	Source(string) Source
}] struct {
	Load                                        func(string, string, func(string) (string, bool)) (S, error)
	NewHandler                                  func(string, string, S) http.Handler
	SignIn                                      func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall                                     AuthCookieRequest
	GUI                                         Source
	SCIMToken, TMDBSection, TMDBLabel, TMDBHelp string
}

func (fixture ConfigurationHTTP[Source, S]) Run(t *testing.T) {
	t.Run("ExternalConfigurationOverridesAndLocksSettings", fixture.ExternalConfigurationOverridesAndLocksSettings)
	t.Run("OwnerCanSaveRestartConfigurationThroughAPI", fixture.OwnerCanSaveRestartConfigurationThroughAPI)
	t.Run("OwnerCanConfigureOIDCSetThroughAPIAndWeb", fixture.OwnerCanConfigureOIDCSetThroughAPIAndWeb)
	t.Run("ManagedOIDCConfigurationIsReadOnlyAndSecret", fixture.ManagedOIDCConfigurationIsReadOnlyAndSecret)
	t.Run("StandaloneOperatorSettingsRemainEditable", fixture.StandaloneOperatorSettingsRemainEditable)
	t.Run("OwnerCanDisableDefaultTLSSettingThroughAPIAndWeb", fixture.OwnerCanDisableDefaultTLSSettingThroughAPIAndWeb)
}

func (fixture ConfigurationHTTP[Source, S]) ExternalConfigurationOverridesAndLocksSettings(t *testing.T) { //nolint:cyclop,funlen // One scenario checks API and web rendering without weakening the secret boundary.
	t.Parallel()
	directory := t.TempDir()
	media := t.TempDir()
	configured, err := fixture.Load(directory, "", func(name string) (string, bool) {
		values := map[string]string{
			"KINOSAIL_SERVER_NAME": "Environment Home", "KINOSAIL_REQUIRE_MFA": "false", "KINOSAIL_LIBRARIES": `["."]`,
			"KINOSAIL_PLAYBACK_MODE": "direct", "KINOSAIL_SUBTITLE_LANGUAGE": "es", "KINOSAIL_TRANSCODE_QUALITY": "speed",
			"KINOSAIL_SCAN_FREQUENCY": "off", "KINOSAIL_DLNA_ENABLED": "false", "KINOSAIL_JELLYFIN_ENABLED": "false", "KINOSAIL_TMDB_TOKEN": "hidden-token", "KINOSAIL_SCIM_TOKEN": fixture.SCIMToken,
			"KINOSAIL_SCIM_TOKEN_EXPIRES_AT": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		}
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	settings := APICall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil)
	locked := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/server", map[string]string{"name": "Changed"})
	lockedMFA := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/mfa", map[string]bool{"required": true})
	lockedJellyfin := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/jellyfin", map[string]bool{"enabled": true})
	lockedSCIM := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/integrations.scim.token", map[string]string{"value": strings.Repeat("s", 32), "tokenExpiresAt": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)})
	afterRejectedChanges := APICall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil)
	settingsPage := APICall(t, handler, session.Token, http.MethodGet, "/settings", nil)
	configurationResponse := APICall(t, handler, session.Token, http.MethodGet, "/api/v1/configuration", nil)
	configurationPage := APICall(t, handler, session.Token, http.MethodGet, "/settings/configuration", nil)
	assertExternalSettingsLocked(t, settings, locked, lockedMFA, lockedJellyfin, lockedSCIM, afterRejectedChanges)
	page := settingsPage.Body.String()
	for _, expected := range []string{
		`Configured via Docker: <code>KINOSAIL_REQUIRE_MFA</code>.`, `Configured via Docker: <code>KINOSAIL_SERVER_NAME</code>.`,
		`Configured via Docker: <code>KINOSAIL_PLAYBACK_MODE</code>.`, `Configured via Docker: <code>KINOSAIL_SUBTITLE_LANGUAGE</code>.`,
		`Configured via Docker: <code>KINOSAIL_TRANSCODE_QUALITY</code>.`, `Configured via Docker: <code>KINOSAIL_JELLYFIN_ENABLED</code>.`,
		`Configured via Docker: <code>KINOSAIL_DLNA_ENABLED</code>.`, `Configured via Docker: <code>KINOSAIL_LIBRARIES</code>.`,
		`Configured via Docker: <code>KINOSAIL_SCAN_FREQUENCY</code>.`, `action="/settings/mfa" method="post"><fieldset disabled aria-disabled="true">`,
		`action="/settings/server" method="post"><fieldset disabled aria-disabled="true">`, `action="/settings/playback" method="post"><fieldset disabled aria-disabled="true">`,
		`action="/settings/subtitles" method="post"><fieldset disabled aria-disabled="true">`, `action="/settings/transcoder" method="post"><fieldset disabled aria-disabled="true">`,
		`action="/settings/jellyfin" method="post"><fieldset disabled aria-disabled="true">`, `action="/settings/libraries" method="post"><fieldset disabled aria-disabled="true">`,
		`action="/settings/scans" method="post"><fieldset disabled aria-disabled="true">`,
		fixture.TMDBSection, `href="/settings/configuration#integrations.oidc"`,
		`href="/settings/configuration#integrations.scim"`, `href="/settings/configuration#integrations.webhook.url"`,
		`href="/settings/configuration#dlna.url"`, `href="/settings/configuration#backup.key"`,
		`SCIM provisioning: <strong class="status">Enabled</strong>`, `Configured via Docker: <code>KINOSAIL_TMDB_TOKEN</code>.`,
	} {
		if settingsPage.Code != http.StatusOK || !strings.Contains(page, expected) {
			t.Fatalf("settings page lacks %q: %d %q", expected, settingsPage.Code, page)
		}
	}
	if strings.Contains(configurationResponse.Body.String(), "hidden-token") || !strings.Contains(configurationResponse.Body.String(), `"source":"environment"`) || !strings.Contains(configurationResponse.Body.String(), `"configured":true`) {
		t.Fatalf("configuration leaked or lacks source: %q", configurationResponse.Body.String())
	}
	configurationHTML := configurationPage.Body.String()
	if configurationPage.Code != http.StatusOK || strings.Contains(configurationHTML, "hidden-token") || !strings.Contains(configurationHTML, fixture.TMDBLabel) || !strings.Contains(configurationHTML, "Configured via Docker: <code>KINOSAIL_TMDB_TOKEN</code>.") || !strings.Contains(configurationHTML, "Configured via Docker: <code>KINOSAIL_SCIM_TOKEN</code>.") || !strings.Contains(configurationHTML, fixture.TMDBHelp) {
		t.Fatalf("configuration page = %d %q", configurationPage.Code, configurationHTML)
	}
}

func (fixture ConfigurationHTTP[Source, S]) OwnerCanSaveRestartConfigurationThroughAPI(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler("", directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	saved := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/backup.retention", map[string]string{"value": "12"})
	wrongOperation := APICall(t, handler, session.Token, http.MethodDelete, "/api/v1/configuration/server.name", nil)
	reloaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusAccepted || wrongOperation.Code != http.StatusConflict || loadErr != nil || reloaded.Int("backup.retention") != 12 || reloaded.Source("backup.retention") != fixture.GUI {
		t.Fatalf("saved=%d %q wrong=%d %q value=%d source=%q err=%v", saved.Code, saved.Body.String(), wrongOperation.Code, wrongOperation.Body.String(), reloaded.Int("backup.retention"), reloaded.Source("backup.retention"), loadErr)
	}
}

func (fixture ConfigurationHTTP[Source, S]) OwnerCanConfigureOIDCSetThroughAPIAndWeb(t *testing.T) { //nolint:cyclop,funlen,gocognit // API, web, secret retention, reset, and no-side-effect parity are one boundary.
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler("", directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	issuer, clientID, redirectURL := "https://identity.example/realms/family", "kinosail", "https://media.example/login/oidc/callback"
	secret := strings.Repeat("s", 24)
	invalid := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer})
	if invalid.Code != http.StatusConflict {
		t.Fatalf("incomplete OIDC API configuration = %d %q", invalid.Code, invalid.Body.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
		t.Fatalf("incomplete OIDC API configuration left regular state: %v", err)
	}
	saved := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer, "clientId": clientID, "clientSecret": secret, "redirectUrl": redirectURL, "identityClaim": "oid"})
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusAccepted || loadErr != nil || loaded.String("integrations.oidc.issuer") != issuer || loaded.String("integrations.oidc.client_id") != clientID || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.redirect_url") != redirectURL || loaded.String("integrations.oidc.identity_claim") != "oid" {
		t.Fatalf("saved=%d %q issuer=%q client=%q secret=%q redirect=%q err=%v", saved.Code, saved.Body.String(), loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_id"), loaded.String("integrations.oidc.client_secret"), loaded.String("integrations.oidc.redirect_url"), loadErr)
	}
	page := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	body := page.Body.String()
	for _, expected := range []string{`id="integrations.oidc"`, `Register it exactly with the provider`, `/login/oidc/callback`, `aria-label="Single sign-on issuer"`, `aria-label="Single sign-on client ID"`, `aria-label="Single sign-on client secret"`, `aria-label="Single sign-on return address"`, `aria-label="Single sign-on identity claim"`, `value="oid"`, `SCIM <code>externalId</code>`, `Leave blank to keep the configured secret`} {
		if page.Code != http.StatusOK || !strings.Contains(body, expected) {
			t.Fatalf("OIDC configuration page lacks %q: %d %q", expected, page.Code, body)
		}
	}
	if strings.Contains(body, secret) || strings.Contains(body, `id="integrations.oidc.issuer"`) {
		t.Fatalf("OIDC configuration leaked a secret or rendered individual fields: %q", body)
	}
	fixture.assertOIDCWebChanges(t, handler, owner, directory, issuer, clientID, secret, redirectURL)
}

func (fixture ConfigurationHTTP[Source, S]) assertOIDCWebChanges(t *testing.T, handler http.Handler, owner *http.Cookie, directory, issuer, clientID, secret, redirectURL string) {
	t.Helper()
	newIssuer, newClientID := "https://login.example/application/o/kinosail", "kinosail-web"
	web := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {newIssuer}, "clientId": {newClientID}, "clientSecret": {""}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}})
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if web.Code != http.StatusSeeOther || loadErr != nil || loaded.String("integrations.oidc.issuer") != newIssuer || loaded.String("integrations.oidc.client_id") != newClientID || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.identity_claim") != "sub" {
		t.Fatalf("web=%d issuer=%q client=%q secret=%q err=%v", web.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_id"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
	fixture.assertOIDCInvalidChanges(t, handler, owner, directory, issuer, clientID, secret, redirectURL, newIssuer)
	fixture.assertOIDCRemoved(t, handler, owner, directory)
}

func (fixture ConfigurationHTTP[Source, S]) assertOIDCInvalidChanges(t *testing.T, handler http.Handler, owner *http.Cookie, directory, issuer, clientID, secret, redirectURL, newIssuer string) {
	t.Helper()
	insecure := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {"http://identity.example"}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}})
	unknown := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {issuer}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}, "unknown": {"true"}})
	invalidClaim := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer, "clientId": clientID, "clientSecret": secret, "redirectUrl": redirectURL, "identityClaim": "object id"})
	oversizedClaim := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {issuer}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {strings.Repeat("x", 257)}})
	individual := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc.issuer", map[string]string{"value": issuer})
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if insecure.Code != http.StatusConflict || unknown.Code != http.StatusBadRequest || invalidClaim.Code != http.StatusConflict || oversizedClaim.Code != http.StatusBadRequest || individual.Code != http.StatusConflict || loadErr != nil || loaded.String("integrations.oidc.issuer") != newIssuer || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.identity_claim") != "sub" {
		t.Fatalf("invalid OIDC inputs insecure=%d unknown=%d claim=%d oversized=%d individual=%d issuer=%q secret=%q err=%v", insecure.Code, unknown.Code, invalidClaim.Code, oversizedClaim.Code, individual.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
}

func (fixture ConfigurationHTTP[Source, S]) assertOIDCRemoved(t *testing.T, handler http.Handler, owner *http.Cookie, directory string) {
	t.Helper()
	removed := APICall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.oidc", nil)
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if removed.Code != http.StatusAccepted || loadErr != nil || loaded.String("integrations.oidc.issuer") != "" || loaded.String("integrations.oidc.client_secret") != "" {
		t.Fatalf("removed=%d issuer=%q secret=%q err=%v", removed.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
}

func (fixture ConfigurationHTTP[Source, S]) ManagedOIDCConfigurationIsReadOnlyAndSecret(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	values := map[string]string{
		"KINOSAIL_OIDC_ISSUER":        "https://identity.example",
		"KINOSAIL_OIDC_CLIENT_ID":     "kinosail",
		"KINOSAIL_OIDC_CLIENT_SECRET": "provider-secret",
		"KINOSAIL_OIDC_REDIRECT_URL":  "https://media.example/login/oidc/callback",
	}
	configured, err := fixture.Load(directory, "", func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler("", directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	page := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	body := page.Body.String()
	for key := range values {
		if page.Code != http.StatusOK || !strings.Contains(body, `<code>`+key+`</code>`) {
			t.Fatalf("managed OIDC page lacks %q: %d %q", key, page.Code, body)
		}
	}
	if strings.Contains(body, values["KINOSAIL_OIDC_CLIENT_SECRET"]) || strings.Contains(body, `name="clientSecret"`) {
		t.Fatalf("managed OIDC page leaked or edited its secret: %q", body)
	}
	changed := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": "https://other.example", "clientId": "other", "clientSecret": "other-secret", "redirectUrl": "https://other.example/login/oidc/callback"})
	if changed.Code != http.StatusConflict {
		t.Fatalf("managed OIDC update = %d %q", changed.Code, changed.Body.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
		t.Fatalf("managed OIDC update left stored state: %v", err)
	}
}

func (fixture ConfigurationHTTP[Source, S]) StandaloneOperatorSettingsRemainEditable(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler("", directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	settingsPage := fixture.WebCall(t, handler, http.MethodGet, "/settings", "", owner)
	if settingsPage.Code != http.StatusOK || strings.Contains(settingsPage.Body.String(), "managed-setting") || !strings.Contains(settingsPage.Body.String(), `action="/settings/server" method="post"`) || !strings.Contains(settingsPage.Body.String(), `action="/settings/jellyfin" method="post"`) || !strings.Contains(settingsPage.Body.String(), `fieldset disabled aria-disabled="true"`) {
		t.Fatalf("standalone settings = %d %q", settingsPage.Code, settingsPage.Body.String())
	}
}

func (fixture ConfigurationHTTP[Source, S]) OwnerCanDisableDefaultTLSSettingThroughAPIAndWeb(t *testing.T) { //nolint:cyclop // One adapter-parity scenario checks the API, web view, persistence, and default.
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || !configured.Bool("tls.enabled") {
		t.Fatalf("default TLS = %v, %v", configured.Bool("tls.enabled"), err)
	}
	handler := fixture.NewHandler("", directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	api := APICall(t, handler, session.Token, http.MethodGet, "/api/v1/configuration", nil)
	web := APICall(t, handler, session.Token, http.MethodGet, "/settings/configuration", nil)
	saved := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/tls.enabled", map[string]string{"value": "false"})
	reloaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	page := web.Body.String()
	var listed struct {
		Settings []struct{ Key, Env, Value string }
	}
	if err := json.Unmarshal(api.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	tlsDefault := false
	for _, setting := range listed.Settings {
		if setting.Key == "tls.enabled" {
			tlsDefault = setting.Env == "KINOSAIL_TLS_ENABLED" && setting.Value == "true"
		}
	}
	for _, fragment := range []string{`<h2>HTTPS enabled</h2>`, `aria-label="HTTPS enabled"`, "Using the Kinosail default.", "Configuration key: <code>tls.enabled</code>"} {
		if !strings.Contains(page, fragment) {
			t.Errorf("configuration page missing %q", fragment)
		}
	}
	if api.Code != http.StatusOK || !tlsDefault || web.Code != http.StatusOK || strings.Contains(page, `<h2><code>tls.enabled</code></h2>`) || strings.Contains(page, `<h2></h2>`) || saved.Code != http.StatusAccepted || loadErr != nil || reloaded.Bool("tls.enabled") || reloaded.Source("tls.enabled") != fixture.GUI {
		t.Fatalf("api=%d default=%v web=%d rawHeading=%v emptyHeading=%v saved=%d TLS=%v source=%q err=%v", api.Code, tlsDefault, web.Code, strings.Contains(page, `<h2><code>tls.enabled</code></h2>`), strings.Contains(page, `<h2></h2>`), saved.Code, reloaded.Bool("tls.enabled"), reloaded.Source("tls.enabled"), loadErr)
	}
}

func assertExternalSettingsLocked(t *testing.T, settings, locked, lockedMFA, lockedJellyfin, lockedSCIM, afterRejectedChanges *httptest.ResponseRecorder) {
	t.Helper()
	AssertAPIBody(t, settings, http.StatusOK, `"name":"Environment Home"`, `"requireMfa":false`, `"jellyfinCompatibility":false`)
	for _, response := range []*httptest.ResponseRecorder{locked, lockedMFA, lockedJellyfin, lockedSCIM} {
		AssertAPIBody(t, response, http.StatusConflict)
	}
	AssertAPIBody(t, afterRejectedChanges, http.StatusOK, `"requireMfa":false`, `"jellyfinCompatibility":false`)
}
