package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestExternalConfigurationOverridesAndLocksSettings(t *testing.T) { //nolint:cyclop,funlen // One scenario checks API and web rendering without weakening the secret boundary.
	t.Parallel()
	directory := t.TempDir()
	media := t.TempDir()
	configured, err := configuration.Load(directory, "", func(name string) (string, bool) {
		values := map[string]string{
			"KINOSAIL_SERVER_NAME": "Environment Home", "KINOSAIL_REQUIRE_MFA": "false", "KINOSAIL_LIBRARIES": `["."]`,
			"KINOSAIL_PLAYBACK_MODE": "direct", "KINOSAIL_SUBTITLE_LANGUAGE": "es", "KINOSAIL_TRANSCODE_QUALITY": "speed",
			"KINOSAIL_SCAN_FREQUENCY": "off", "KINOSAIL_DLNA_ENABLED": "false", "KINOSAIL_JELLYFIN_ENABLED": "false", "KINOSAIL_TMDB_TOKEN": "hidden-token", "KINOSAIL_SCIM_TOKEN": scimTestToken,
			"KINOSAIL_SCIM_TOKEN_EXPIRES_AT": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		}
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	settings := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil)
	locked := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/server", map[string]string{"name": "Changed"})
	lockedMFA := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/mfa", map[string]bool{"required": true})
	lockedJellyfin := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/settings/jellyfin", map[string]bool{"enabled": true})
	lockedSCIM := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/integrations.scim.token", map[string]string{"value": strings.Repeat("s", 32), "tokenExpiresAt": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)})
	afterRejectedChanges := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil)
	settingsPage := apiCall(t, handler, session.Token, http.MethodGet, "/settings", nil)
	configurationResponse := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/configuration", nil)
	configurationPage := apiCall(t, handler, session.Token, http.MethodGet, "/settings/configuration", nil)
	if !strings.Contains(settings.Body.String(), `"name":"Environment Home"`) || !strings.Contains(settings.Body.String(), `"requireMfa":false`) || !strings.Contains(settings.Body.String(), `"jellyfinCompatibility":false`) || locked.Code != http.StatusConflict || lockedMFA.Code != http.StatusConflict || lockedJellyfin.Code != http.StatusConflict || lockedSCIM.Code != http.StatusConflict || afterRejectedChanges.Code != http.StatusOK || !strings.Contains(afterRejectedChanges.Body.String(), `"requireMfa":false`) || !strings.Contains(afterRejectedChanges.Body.String(), `"jellyfinCompatibility":false`) {
		t.Fatalf("settings=%d %q locked=%d %q MFA=%d Jellyfin=%d SCIM=%d after=%d %q", settings.Code, settings.Body.String(), locked.Code, locked.Body.String(), lockedMFA.Code, lockedJellyfin.Code, lockedSCIM.Code, afterRejectedChanges.Code, afterRejectedChanges.Body.String())
	}
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
		`href="/settings/configuration#integrations.tmdb.token"`, `href="/settings/configuration#integrations.oidc"`,
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
	if configurationPage.Code != http.StatusOK || strings.Contains(configurationHTML, "hidden-token") || !strings.Contains(configurationHTML, "TMDB access token") || !strings.Contains(configurationHTML, "Configured via Docker: <code>KINOSAIL_TMDB_TOKEN</code>.") || !strings.Contains(configurationHTML, "Configured via Docker: <code>KINOSAIL_SCIM_TOKEN</code>.") || !strings.Contains(configurationHTML, "Environment variable: <code>KINOSAIL_TMDB_TOKEN</code>") {
		t.Fatalf("configuration page = %d %q", configurationPage.Code, configurationHTML)
	}
}

func TestOwnerCanSaveRestartConfigurationThroughAPI(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	saved := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/backup.retention", map[string]string{"value": "12"})
	wrongOperation := apiCall(t, handler, session.Token, http.MethodDelete, "/api/v1/configuration/server.name", nil)
	reloaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusAccepted || wrongOperation.Code != http.StatusConflict || loadErr != nil || reloaded.Int("backup.retention") != 12 || reloaded.Source("backup.retention") != configuration.GUI {
		t.Fatalf("saved=%d %q wrong=%d %q value=%d source=%q err=%v", saved.Code, saved.Body.String(), wrongOperation.Code, wrongOperation.Body.String(), reloaded.Int("backup.retention"), reloaded.Source("backup.retention"), loadErr)
	}
}

func TestOwnerCanConfigureOIDCSetThroughAPIAndWeb(t *testing.T) { //nolint:cyclop,funlen,gocognit // API, web, secret retention, reset, and no-side-effect parity are one boundary.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	issuer, clientID, redirectURL := "https://identity.example/realms/family", "kinosail", "https://media.example/login/oidc/callback"
	secret := strings.Repeat("s", 24)
	invalid := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer})
	if invalid.Code != http.StatusConflict {
		t.Fatalf("incomplete OIDC API configuration = %d %q", invalid.Code, invalid.Body.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
		t.Fatalf("incomplete OIDC API configuration left regular state: %v", err)
	}
	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer, "clientId": clientID, "clientSecret": secret, "redirectUrl": redirectURL, "identityClaim": "oid"})
	loaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusAccepted || loadErr != nil || loaded.String("integrations.oidc.issuer") != issuer || loaded.String("integrations.oidc.client_id") != clientID || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.redirect_url") != redirectURL || loaded.String("integrations.oidc.identity_claim") != "oid" {
		t.Fatalf("saved=%d %q issuer=%q client=%q secret=%q redirect=%q err=%v", saved.Code, saved.Body.String(), loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_id"), loaded.String("integrations.oidc.client_secret"), loaded.String("integrations.oidc.redirect_url"), loadErr)
	}
	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	body := page.Body.String()
	for _, expected := range []string{`id="integrations.oidc"`, `Register it exactly with the provider`, `/login/oidc/callback`, `aria-label="Single sign-on issuer"`, `aria-label="Single sign-on client ID"`, `aria-label="Single sign-on client secret"`, `aria-label="Single sign-on return address"`, `aria-label="Single sign-on identity claim"`, `value="oid"`, `SCIM <code>externalId</code>`, `Leave blank to keep the configured secret`} {
		if page.Code != http.StatusOK || !strings.Contains(body, expected) {
			t.Fatalf("OIDC configuration page lacks %q: %d %q", expected, page.Code, body)
		}
	}
	if strings.Contains(body, secret) || strings.Contains(body, `id="integrations.oidc.issuer"`) {
		t.Fatalf("OIDC configuration leaked a secret or rendered individual fields: %q", body)
	}
	newIssuer, newClientID := "https://login.example/application/o/kinosail", "kinosail-web"
	web := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {newIssuer}, "clientId": {newClientID}, "clientSecret": {""}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}})
	loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if web.Code != http.StatusSeeOther || loadErr != nil || loaded.String("integrations.oidc.issuer") != newIssuer || loaded.String("integrations.oidc.client_id") != newClientID || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.identity_claim") != "sub" {
		t.Fatalf("web=%d issuer=%q client=%q secret=%q err=%v", web.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_id"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
	insecure := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {"http://identity.example"}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}})
	unknown := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {issuer}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {"sub"}, "unknown": {"true"}})
	invalidClaim := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": issuer, "clientId": clientID, "clientSecret": secret, "redirectUrl": redirectURL, "identityClaim": "object id"})
	oversizedClaim := webFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.oidc"}, "issuer": {issuer}, "clientId": {clientID}, "clientSecret": {secret}, "redirectUrl": {redirectURL}, "identityClaim": {strings.Repeat("x", 257)}})
	individual := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc.issuer", map[string]string{"value": issuer})
	loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if insecure.Code != http.StatusConflict || unknown.Code != http.StatusBadRequest || invalidClaim.Code != http.StatusConflict || oversizedClaim.Code != http.StatusBadRequest || individual.Code != http.StatusConflict || loadErr != nil || loaded.String("integrations.oidc.issuer") != newIssuer || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.identity_claim") != "sub" {
		t.Fatalf("invalid OIDC inputs insecure=%d unknown=%d claim=%d oversized=%d individual=%d issuer=%q secret=%q err=%v", insecure.Code, unknown.Code, invalidClaim.Code, oversizedClaim.Code, individual.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
	removed := apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/configuration/integrations.oidc", nil)
	loaded, loadErr = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if removed.Code != http.StatusAccepted || loadErr != nil || loaded.String("integrations.oidc.issuer") != "" || loaded.String("integrations.oidc.client_secret") != "" {
		t.Fatalf("removed=%d issuer=%q secret=%q err=%v", removed.Code, loaded.String("integrations.oidc.issuer"), loaded.String("integrations.oidc.client_secret"), loadErr)
	}
}

func TestManagedOIDCConfigurationIsReadOnlyAndSecret(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	values := map[string]string{
		"KINOSAIL_OIDC_ISSUER":        "https://identity.example",
		"KINOSAIL_OIDC_CLIENT_ID":     "kinosail",
		"KINOSAIL_OIDC_CLIENT_SECRET": "provider-secret",
		"KINOSAIL_OIDC_REDIRECT_URL":  "https://media.example/login/oidc/callback",
	}
	configured, err := configuration.Load(directory, "", func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	body := page.Body.String()
	for key := range values {
		if page.Code != http.StatusOK || !strings.Contains(body, `<code>`+key+`</code>`) {
			t.Fatalf("managed OIDC page lacks %q: %d %q", key, page.Code, body)
		}
	}
	if strings.Contains(body, values["KINOSAIL_OIDC_CLIENT_SECRET"]) || strings.Contains(body, `name="clientSecret"`) {
		t.Fatalf("managed OIDC page leaked or edited its secret: %q", body)
	}
	changed := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.oidc", map[string]string{"issuer": "https://other.example", "clientId": "other", "clientSecret": "other-secret", "redirectUrl": "https://other.example/login/oidc/callback"})
	if changed.Code != http.StatusConflict {
		t.Fatalf("managed OIDC update = %d %q", changed.Code, changed.Body.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
		t.Fatalf("managed OIDC update left stored state: %v", err)
	}
}

func TestStandaloneOperatorSettingsRemainEditable(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	settingsPage := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	if settingsPage.Code != http.StatusOK || strings.Contains(settingsPage.Body.String(), "managed-setting") || !strings.Contains(settingsPage.Body.String(), `action="/settings/server" method="post"`) || !strings.Contains(settingsPage.Body.String(), `action="/settings/jellyfin" method="post"`) || !strings.Contains(settingsPage.Body.String(), `fieldset disabled aria-disabled="true"`) {
		t.Fatalf("standalone settings = %d %q", settingsPage.Code, settingsPage.Body.String())
	}
}

func TestOwnerCanDisableDefaultTLSSettingThroughAPIAndWeb(t *testing.T) { //nolint:cyclop // One adapter-parity scenario checks the API, web view, persistence, and default.
	t.Parallel()
	directory := t.TempDir()
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || !configured.Bool("tls.enabled") {
		t.Fatalf("default TLS = %v, %v", configured.Bool("tls.enabled"), err)
	}
	handler := server.New(server.Config{DataDir: directory, RequireAuth: true, Configuration: configured})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	api := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/configuration", nil)
	web := apiCall(t, handler, session.Token, http.MethodGet, "/settings/configuration", nil)
	saved := apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/configuration/tls.enabled", map[string]string{"value": "false"})
	reloaded, loadErr := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	page := web.Body.String()
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"key":"tls.enabled","env":"KINOSAIL_TLS_ENABLED","value":"true"`) || web.Code != http.StatusOK || !strings.Contains(page, `<h2>HTTPS enabled</h2>`) || !strings.Contains(page, `aria-label="HTTPS enabled"`) || !strings.Contains(page, "Using the Kinosail default.") || !strings.Contains(page, "Configuration key: <code>tls.enabled</code>") || strings.Contains(page, `<h2><code>tls.enabled</code></h2>`) || strings.Contains(page, `<h2></h2>`) || saved.Code != http.StatusAccepted || loadErr != nil || reloaded.Bool("tls.enabled") || reloaded.Source("tls.enabled") != configuration.GUI {
		t.Fatalf("api=%d %q web=%d saved=%d TLS=%v source=%q err=%v", api.Code, api.Body.String(), web.Code, saved.Code, reloaded.Bool("tls.enabled"), reloaded.Source("tls.enabled"), loadErr)
	}
}
