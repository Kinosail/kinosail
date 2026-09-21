package appcli

import (
	"strconv"
	"testing"
	"time"
)

type serverTestSettings struct{ testSettings }

func (s serverTestSettings) Int(key string) int {
	value, _ := strconv.Atoi(s.String(key))
	return value
}

func (s serverTestSettings) Duration(key string) time.Duration {
	value, _ := time.ParseDuration(s.String(key))
	return value
}

func TestServerValuesKeepIdentityStorageAndExpiryBoundaries(t *testing.T) {
	settings := serverTestSettings{testSettings{ //nolint:gosec // G101: synthetic values verify private configuration mapping; no real credentials.
		"paths.data": "/private/state", "paths.media": "/library", "paths.cache": "/cache",
		"backup.interval": "24h", "backup.retention": "7", "integrations.scim.token": "private-token",
		"integrations.scim.token_expires_at": "2027-01-02T03:04:05Z", "integrations.oidc.client_secret": "client-secret",
	}}
	values := BuildServerValues(settings, "https://media.example.test")
	assertIdentityServerValues(t, values)
	if values.MediaDir != "/library" || values.CacheDir != "/cache" || values.BackupInterval != 24*time.Hour || values.BackupRetention != 7 {
		t.Fatal("server storage and backup policy were not retained")
	}
	settings.testSettings["integrations.scim.token_expires_at"] = "invalid"
	if !BuildServerValues(settings, "").SCIM.TokenExpiresAt.IsZero() {
		t.Fatal("malformed expiry became a valid timestamp")
	}
}

func assertIdentityServerValues(t *testing.T, values ServerValues) {
	t.Helper()
	if values.SAML.DataDir != "/private/state" || values.SAML.RootURL != values.AuthURL || values.AuthURL != "https://media.example.test" {
		t.Fatal("identity provider lost app storage or callback origin")
	}
	if values.SCIM.Token != "private-token" || values.OIDC.ClientSecret != "client-secret" || !values.SCIM.TokenExpiresAt.Equal(time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatal("identity credentials or expiry changed")
	}
}
