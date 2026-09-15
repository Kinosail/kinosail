package federation

import (
	"errors"
	"maps"
	"strings"
	"sync"
	"testing"
)

type settingsEffects struct {
	regular, secrets map[string]string
	reads, writes    int
	persistErr       error
}

func TestSettingsOIDCLifecycleAndValidationEffects(t *testing.T) { //nolint:cyclop // One lifecycle proves each mutation boundary.
	t.Parallel()
	effects := &settingsEffects{}
	settings := testSettings(effects)
	invalid := OIDCConfig{Issuer: "http://identity.example", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://media.example/callback"}
	if err := settings.SetOIDC(invalid); err == nil || effects.reads != 0 || effects.writes != 0 {
		t.Fatalf("invalid OIDC effects = %v %#v", err, effects)
	}
	valid := OIDCConfig{Issuer: "https://identity.example", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://media.example/callback"}
	if err := settings.SetOIDC(valid); err != nil {
		t.Fatal(err)
	}
	if effects.regular[OIDCIdentityKey] != "sub" || effects.secrets[OIDCSecretKey] != "secret" || effects.writes != 1 {
		t.Fatalf("saved OIDC = %#v %#v", effects.regular, effects.secrets)
	}
	if err := settings.DeleteOIDC(); err != nil || IsOIDCKey("other") || !IsOIDCKey(OIDCIssuerKey) {
		t.Fatalf("delete OIDC = %v %#v %#v", err, effects.regular, effects.secrets)
	}
	if effects.regular[OIDCIssuerKey] != "" || effects.secrets[OIDCSecretKey] != "" {
		t.Fatal("deleted OIDC configuration remained")
	}
}

func TestSettingsSAMLLifecycleAndPersistenceErrors(t *testing.T) {
	t.Parallel()
	effects := &settingsEffects{}
	settings := testSettings(effects)
	if err := settings.SetSAML(SAMLConfig{}); err == nil || effects.reads != 0 {
		t.Fatalf("missing SAML source effects = %v %#v", err, effects)
	}
	if err := settings.SetSAML(SAMLConfig{MetadataURL: "https://identity.example/metadata", IdentityAttribute: ""}); err != nil {
		t.Fatal(err)
	}
	if effects.regular[SAMLIdentityKey] != "NameID" || !IsSAMLKey(SAMLMetadataURLKey) || IsSAMLKey("other") {
		t.Fatalf("saved SAML = %#v", effects.regular)
	}
	effects.persistErr = errors.New("disk failed")
	if err := settings.DeleteSAML(); err == nil || !strings.Contains(err.Error(), "save SAML configuration") {
		t.Fatalf("delete SAML error = %v", err)
	}
}

func TestSettingsCrossFieldAndTextValidation(t *testing.T) { //nolint:cyclop // Related validators share one compact table-free contract.
	t.Parallel()
	values := map[string]string{}
	value := func(key string) string { return values[key] }
	if ValidateOIDC(value) != nil || ValidateSAML(value) != nil {
		t.Fatal("empty federation settings were rejected")
	}
	values[OIDCIssuerKey] = "https://identity.example"
	if ValidateOIDC(value) == nil {
		t.Fatal("partial OIDC settings were accepted")
	}
	values = map[string]string{SAMLMetadataURLKey: "https://identity.example/metadata", SAMLMetadataXMLKey: "xml"}
	if ValidateSAML(value) == nil {
		t.Fatal("conflicting SAML settings were accepted")
	}
	if !BoundedIdentityText("client", 6) || BoundedIdentityText("bad\n", 6) || BoundedIdentityText(string([]byte{0xff}), 6) {
		t.Fatal("bounded identity text validation is invalid")
	}
	if !ValidIdentityField("objectGUID") || ValidIdentityField("object id") || ValidIdentityField(strings.Repeat("x", 257)) {
		t.Fatal("identity field validation is invalid")
	}
}

func testSettings(effects *settingsEffects) Settings { //nolint:cyclop,gocognit // Test callbacks intentionally expose each storage effect.
	var mutex sync.Mutex
	if effects.regular == nil {
		effects.regular = make(map[string]string)
	}
	if effects.secrets == nil {
		effects.secrets = make(map[string]string)
	}
	validateValue := func(key, value string) error {
		switch key {
		case OIDCIssuerKey, OIDCRedirectKey, SAMLMetadataURLKey:
			if value != "" && !ValidIdentityURL(value) {
				return errors.New("invalid URL")
			}
		case OIDCClientIDKey, OIDCSecretKey:
			if value == "" {
				return errors.New("missing client value")
			}
		case OIDCIdentityKey, SAMLIdentityKey:
			if !ValidIdentityField(value) {
				return errors.New("invalid identity field")
			}
		}
		return nil
	}
	read := func(string) (map[string]string, map[string]string, error) {
		effects.reads++
		return maps.Clone(effects.regular), maps.Clone(effects.secrets), nil
	}
	persist := func(_ string, regular, secrets map[string]string, _, _ bool) error {
		effects.writes++
		if effects.persistErr == nil {
			effects.regular, effects.secrets = maps.Clone(regular), maps.Clone(secrets)
		}
		return effects.persistErr
	}
	return NewSettings("data", &mutex, validateValue, read, func(map[string]string, map[string]string) error { return nil }, persist)
}
