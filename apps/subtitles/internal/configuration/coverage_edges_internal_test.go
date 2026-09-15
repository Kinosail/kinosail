package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func TestStoredConfigurationValidationEdges(t *testing.T) { //nolint:cyclop // Each case targets one independent stored-state policy.
	for name, stored := range map[string]struct {
		regular map[string]string
		secrets map[string]string
	}{
		"unknown key":           {regular: map[string]string{"unknown": "value"}},
		"invalid value":         {regular: map[string]string{"server.name": ""}},
		"SCIM expiration":       {regular: map[string]string{scimExpirationKey: "2030-01-01T00:00:00Z"}},
		"OIDC partial":          {regular: map[string]string{"integrations.oidc.issuer": "https://identity.example"}},
		"OpenSubtitles partial": {secrets: map[string]string{openSubtitlesAPIKey: "key"}},
		"SubSource partial":     {regular: map[string]string{subSourcePersonalUse: "true"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateStoredConfiguration(stored.regular, stored.secrets); err == nil {
				t.Fatal("invalid stored configuration was accepted")
			}
		})
	}

	config, err := trustedhttps.NewProviderConfig(trustedhttps.ProviderDuckDNS, "family", strings.Repeat("t", 32), "192.168.1.10", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := config.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateStoredConfiguration(map[string]string{"tls.enabled": "false"}, map[string]string{"tls.duckdns": raw}); err == nil {
		t.Fatal("stored trusted HTTPS without TLS was accepted")
	}
}

func TestStoredConfigurationReadAndValidationHelpers(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, configurationFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, secretsFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readStoredConfiguration(directory); err == nil {
		t.Fatal("invalid secret configuration file was accepted")
	}
	if err := coupledSettingError("integrations.saml.metadata_url"); err == nil {
		t.Fatal("individual SAML setting was accepted")
	}
	if err := coupledSettingError(subSourceAPIKey); err == nil {
		t.Fatal("individual SubSource setting was accepted")
	}
	expiresOnly := Snapshot{values: map[string]value{scimExpirationKey: {raw: "2030-01-01T00:00:00Z"}}}
	if err := validateSCIM(expiresOnly); err == nil {
		t.Fatal("SCIM expiration without token was accepted")
	}
}

func TestConfigurationValueValidationEdges(t *testing.T) {
	if err := validateKind(kind("unsupported"), "value"); err == nil {
		t.Fatal("unknown configuration kind was accepted")
	}
	if err := validateServerName(""); err == nil {
		t.Fatal("empty server name was accepted")
	}
	if err := validatePositiveDuration("0s"); err == nil {
		t.Fatal("non-positive duration was accepted")
	}
	if err := validatePositiveDuration("1s"); err != nil {
		t.Fatalf("positive duration = %v", err)
	}
}
