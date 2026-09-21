package configurationcore

import "testing"

func TestCommonApplicationFieldsPreservePrivateCredentialsAndAppDefaults(t *testing.T) {
	fields := CommonApplicationFields(":1234", "https://support.example.test")
	byKey, environments := map[string]ApplicationField{}, map[string]bool{}
	for _, field := range fields {
		if field.Key == "" || field.Env == "" || byKey[field.Key].Key != "" || environments[field.Env] {
			t.Fatalf("ambiguous configuration field %q", field.Key)
		}
		byKey[field.Key], environments[field.Env] = field, true
	}
	if byKey["listen"].Default != ":1234" {
		t.Fatal("shared catalog replaced the app's listen address")
	}
	assertPrivateApplicationCredentials(t, byKey)
	fields[0].Default = "changed"
	if CommonApplicationFields(":5678", "")[0].Default != ":5678" {
		t.Fatal("app configuration catalogs share mutable state")
	}
}

func assertPrivateApplicationCredentials(t *testing.T, byKey map[string]ApplicationField) {
	t.Helper()
	for _, key := range []string{"tls.duckdns", "integrations.tmdb.token", "integrations.oidc.client_secret", "integrations.scim.token", "integrations.mcp.client_secret", "integrations.webhook.token"} {
		if field := byKey[key]; !field.Secret || field.Kind != "text" || !field.Restart {
			t.Fatalf("credential %q lost its protection or lifecycle metadata", key)
		}
	}
}
