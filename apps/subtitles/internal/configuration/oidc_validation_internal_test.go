package configuration

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
)

func TestValidOIDCURLAllowsOnlyTrustedEndpointForms(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://identity.example":                   true,
		"http://localhost:8080":                      true,
		"http://127.0.0.1:8080/callback":             true,
		"":                                           false,
		"identity.example":                           false,
		"http://identity.example":                    false,
		"https://user:password@identity.example":     false,
		"https://identity.example?tenant=family":     false,
		"https://identity.example/callback#fragment": false,
	} {
		if got := federation.ValidIdentityURL(raw); got != want {
			t.Errorf("validOIDCURL(%q) = %t, want %t", raw, got, want)
		}
	}
}

func TestValidRemoteTokenRejectsWrongLengthAndCharacters(t *testing.T) {
	valid := "abcdefghijklmnopqrstuvwxyz012345"
	for token, want := range map[string]bool{
		valid:                    true,
		valid + "-_:":            false,
		"short":                  false,
		strings.Repeat("a", 129): false,
	} {
		if got := validRemoteToken(token); got != want {
			t.Errorf("validRemoteToken(%q) = %t, want %t", token, got, want)
		}
	}
}
