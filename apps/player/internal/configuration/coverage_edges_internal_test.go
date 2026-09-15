package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoredConfigurationValidationEdges(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJSON(filepath.Join(parent, "configuration.json")); err == nil {
		t.Fatal("unreadable stored configuration was accepted")
	}
	if err := coupledSettingError("integrations.saml.metadata_url"); err == nil {
		t.Fatal("individual SAML setting was accepted")
	}
	if err := validateStoredConfiguration(map[string]string{"integrations.oidc.issuer": "https://identity.example"}, nil); err == nil {
		t.Fatal("incomplete stored OIDC configuration was accepted")
	}
	expiresOnly := Snapshot{values: map[string]value{scimExpirationKey: {raw: "2030-01-01T00:00:00Z"}}}
	if err := validateSCIM(expiresOnly); err == nil {
		t.Fatal("stored SCIM expiration without a token was accepted")
	}
}
