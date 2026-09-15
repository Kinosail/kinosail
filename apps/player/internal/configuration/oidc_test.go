package configuration_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestOIDCConfigurationRequiresOneCompleteSafeSet(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"KINOSAIL_OIDC_ISSUER":        "https://identity.example/realms/family",
		"KINOSAIL_OIDC_CLIENT_ID":     "kinosail",
		"KINOSAIL_OIDC_CLIENT_SECRET": "provider-secret",
		"KINOSAIL_OIDC_REDIRECT_URL":  "https://media.example/login/oidc/callback",
	}
	if _, err := configuration.Load(t.TempDir(), "", lookup(valid)); err != nil {
		t.Fatalf("valid OIDC configuration: %v", err)
	}
	for name, change := range map[string]map[string]string{
		"missing issuer":      {"KINOSAIL_OIDC_ISSUER": ""},
		"missing client ID":   {"KINOSAIL_OIDC_CLIENT_ID": ""},
		"missing secret":      {"KINOSAIL_OIDC_CLIENT_SECRET": ""},
		"missing return URL":  {"KINOSAIL_OIDC_REDIRECT_URL": ""},
		"insecure issuer":     {"KINOSAIL_OIDC_ISSUER": "http://identity.example"},
		"issuer credentials":  {"KINOSAIL_OIDC_ISSUER": "https://owner@identity.example"},
		"issuer query":        {"KINOSAIL_OIDC_ISSUER": "https://identity.example?tenant=family"},
		"return URL fragment": {"KINOSAIL_OIDC_REDIRECT_URL": "https://media.example/login/oidc/callback#fragment"},
		"oversized client ID": {"KINOSAIL_OIDC_CLIENT_ID": strings.Repeat("x", 513)},
		"oversized secret":    {"KINOSAIL_OIDC_CLIENT_SECRET": strings.Repeat("x", 4097)},
		"spaced claim":        {"KINOSAIL_OIDC_IDENTITY_CLAIM": "object id"},
		"oversized claim":     {"KINOSAIL_OIDC_IDENTITY_CLAIM": strings.Repeat("x", 257)},
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			maps.Copy(values, valid)
			maps.Copy(values, change)
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err == nil {
				t.Fatal("invalid OIDC configuration was accepted")
			}
		})
	}
}

func TestOIDCConfigurationChangesAtomically(t *testing.T) { //nolint:cyclop // One transaction scenario proves save, rejection without side effects, and removal.
	t.Parallel()
	directory := t.TempDir()
	issuer, clientID, redirectURL := "https://identity.example", "kinosail", "https://media.example/login/oidc/callback"
	secret := strings.Repeat("s", 24)
	if err := configuration.SetOIDC(directory, issuer, clientID, secret, redirectURL, "oid"); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.oidc.issuer") != issuer || loaded.String("integrations.oidc.client_id") != clientID || loaded.String("integrations.oidc.client_secret") != secret || loaded.String("integrations.oidc.redirect_url") != redirectURL || loaded.String("integrations.oidc.identity_claim") != "oid" {
		t.Fatalf("saved OIDC configuration = %#v, %v", loaded, err)
	}
	regularBefore, _ := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secretsBefore, _ := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if err := configuration.SetOIDC(directory, issuer, clientID, secret, "http://media.example/login/oidc/callback", "oid"); err == nil {
		t.Fatal("invalid OIDC update was accepted")
	}
	regularAfter, _ := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secretsAfter, _ := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if string(regularAfter) != string(regularBefore) || string(secretsAfter) != string(secretsBefore) {
		t.Fatal("invalid OIDC update changed persisted state")
	}
	if err := configuration.SetOIDC(directory, issuer, clientID, secret, redirectURL, "object id"); err == nil {
		t.Fatal("invalid OIDC identity claim was accepted")
	}
	regularAfter, _ = os.ReadFile(filepath.Join(directory, "configuration.json"))
	secretsAfter, _ = os.ReadFile(filepath.Join(directory, "secrets.json"))
	if string(regularAfter) != string(regularBefore) || string(secretsAfter) != string(secretsBefore) {
		t.Fatal("invalid OIDC identity claim changed persisted state")
	}
	if err := configuration.Set(directory, "integrations.oidc.issuer", issuer); err == nil {
		t.Fatal("individual OIDC update was accepted")
	}
	if err := configuration.DeleteOIDC(directory); err != nil {
		t.Fatal(err)
	}
	loaded, err = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.oidc.issuer") != "" || loaded.String("integrations.oidc.client_secret") != "" {
		t.Fatalf("removed OIDC configuration = %#v, %v", loaded, err)
	}
}
