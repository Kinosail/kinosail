package servertest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertTrustedHTTPSRejectsInvalidInputWithoutSideEffects preserves rejection and secret-file side-effect checks.
func AssertTrustedHTTPSRejectsInvalidInputWithoutSideEffects(t *testing.T, fixture TrustedHTTPSFixture) {
	t.Parallel()
	directory := t.TempDir()
	handler := fixture.New(t, directory)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	for name, input := range map[string]map[string]any{
		"missing token":  {"domain": "family", "address": "192.168.1.10", "termsAccepted": true},
		"wrong hostname": {"domain": "family.example", "token": fixture.Token, "address": "192.168.1.10", "termsAccepted": true},
		"public address": {"domain": "family", "token": fixture.Token, "address": "203.0.113.10", "termsAccepted": true},
		"unsafe token":   {"domain": "family", "token": strings.Repeat("a", 31) + "/", "address": "192.168.1.10", "termsAccepted": true},
		"missing terms":  {"domain": "family", "token": fixture.Token, "address": "192.168.1.10"},
	} {
		t.Run(name, func(t *testing.T) {
			response := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", input)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid input = %d %q", response.Code, response.Body.String())
			}
		})
	}
	duplicate := fixture.Web(t, handler, http.MethodPost, "/settings/trusted-https", "domain=family&domain=other&token="+fixture.Token+"&address=192.168.1.10&termsAccepted=true", owner)
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate web input = %d %q", duplicate.Code, duplicate.Body.String())
	}
	onboardingDuplicate := fixture.Web(t, handler, http.MethodPost, "/onboarding/trusted-https", "domain=family&domain=other&token="+fixture.Token+"&address=192.168.1.10&termsAccepted=true", owner)
	if onboardingDuplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate onboarding input = %d %q", onboardingDuplicate.Code, onboardingDuplicate.Body.String())
	}
	if contents, readErr := os.ReadFile(filepath.Join(directory, "secrets.json")); readErr == nil && strings.Contains(string(contents), "tls.duckdns") {
		t.Fatalf("invalid input changed secrets: %q", contents)
	}
}

// AssertTrustedHTTPSStorageFailureIsGeneric preserves the real storage failure and redaction checks.
func AssertTrustedHTTPSStorageFailureIsGeneric(t *testing.T, fixture TrustedHTTPSFixture) {
	t.Parallel()
	directory := t.TempDir()
	handler := fixture.New(t, directory)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	if err := os.Mkdir(filepath.Join(directory, "secrets.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	response := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family", "token": fixture.Token, "address": "192.168.1.10", "termsAccepted": true})
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), directory) || strings.Contains(response.Body.String(), fixture.Token) || !strings.Contains(response.Body.String(), "storage failed") {
		t.Fatalf("storage failure = %d %q", response.Code, response.Body.String())
	}
}
