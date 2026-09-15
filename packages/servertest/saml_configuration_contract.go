package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SAMLConfiguration binds the settings contracts to actual app loading and authentication.
type SAMLConfiguration[S interface{ String(string) string }] struct {
	Load        func(string, string, func(string) (string, bool)) (S, error)
	New         func(string, S) http.Handler
	SignIn      func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall     func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
	IDPMetadata func(*testing.T, string) string
}

// OwnerCanConfigureSAMLWithProviderMetadataURL checks the actual app's SAML configuration boundary.
func (fixture SAMLConfiguration[S]) OwnerCanConfigureSAMLWithProviderMetadataURL(t *testing.T) {
	t.Helper()
	t.Parallel()
	directory := t.TempDir()
	configured, err := fixture.Load(directory, "", func(name string) (string, bool) {
		return "https://media.example", name == "KINOSAIL_AUTH_URL"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	page := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	for _, expected := range []string{`id="integrations.saml"`, `aria-label="SAML provider metadata URL"`, `aria-label="SAML provider metadata XML"`, `aria-label="SAML identity attribute"`, `value="NameID"`, `SCIM <code>externalId</code>`, `aria-label="Copy service provider metadata URL"`, `aria-label="Copy Assertion Consumer Service URL"`, `rows="6"`, `value="https://media.example/login/saml/metadata"`, `value="https://media.example/login/saml/acs"`} {
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("SAML configuration page lacks %q: %d %q", expected, page.Code, page.Body.String())
		}
	}
	metadataURL := "https://identity.example/application/metadata"
	saved := WebFormCall(t, handler, owner.Value, "/settings/configuration", map[string][]string{"key": {"integrations.saml"}, "metadataUrl": {metadataURL}, "metadataXml": {""}, "identityAttribute": {"objectGUID"}})
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if saved.Code != http.StatusSeeOther || loadErr != nil || loaded.String("integrations.saml.metadata_url") != metadataURL || loaded.String("integrations.saml.identity_attribute") != "objectGUID" {
		t.Fatalf("saved=%d metadata=%q err=%v", saved.Code, loaded.String("integrations.saml.metadata_url"), loadErr)
	}
	fixture.assertRejectedSAMLConfiguration(t, handler, owner.Value, directory, metadataURL)
}

// SettingsShowDownloadedSAMLMetadataAsConfigured checks the actual app's SAML configuration boundary.
func (fixture SAMLConfiguration[S]) SettingsShowDownloadedSAMLMetadataAsConfigured(t *testing.T) {
	t.Helper()
	t.Parallel()
	directory := t.TempDir()
	values := map[string]string{
		"KINOSAIL_AUTH_URL":          "https://media.example",
		"KINOSAIL_SAML_METADATA_XML": strings.ReplaceAll(fixture.IDPMetadata(t, "https://identity.example/metadata"), "IDP_ORIGIN", "https://identity.example"),
	}
	configured, err := fixture.Load(directory, "", func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(directory, configured)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	settings := fixture.WebCall(t, handler, http.MethodGet, "/settings", "", owner)
	configurationPage := fixture.WebCall(t, handler, http.MethodGet, "/settings/configuration", "", owner)
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), `SAML: <strong class="status">Enabled</strong>`) {
		t.Fatalf("SAML settings status = %d %q", settings.Code, settings.Body.String())
	}
	if configurationPage.Code != http.StatusOK || !strings.Contains(configurationPage.Body.String(), "Configured via Docker: <code>KINOSAIL_SAML_METADATA_XML</code>") {
		t.Fatalf("SAML XML control = %d %q", configurationPage.Code, configurationPage.Body.String())
	}
}

func (fixture SAMLConfiguration[S]) assertRejectedSAMLConfiguration(t *testing.T, handler http.Handler, session, directory, metadataURL string) {
	t.Helper()
	insecure := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.saml", map[string]string{"metadataUrl": "http://identity.example/metadata"})
	invalidIdentity := APICall(t, handler, session, http.MethodPut, "/api/v1/configuration/integrations.saml", map[string]string{"metadataUrl": metadataURL, "identityAttribute": "object id"})
	oversizedIdentity := WebFormCall(t, handler, session, "/settings/configuration", map[string][]string{"key": {"integrations.saml"}, "metadataUrl": {metadataURL}, "metadataXml": {""}, "identityAttribute": {strings.Repeat("x", 257)}})
	loaded, loadErr := fixture.Load(directory, "", func(string) (string, bool) { return "", false })
	if insecure.Code != http.StatusConflict || invalidIdentity.Code != http.StatusConflict || oversizedIdentity.Code != http.StatusBadRequest || loadErr != nil || loaded.String("integrations.saml.metadata_url") != metadataURL || loaded.String("integrations.saml.identity_attribute") != "objectGUID" {
		t.Fatalf("insecure=%d identity=%d oversized=%d metadata=%q err=%v", insecure.Code, invalidIdentity.Code, oversizedIdentity.Code, loaded.String("integrations.saml.metadata_url"), loadErr)
	}
}
