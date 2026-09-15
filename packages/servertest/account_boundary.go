package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AccountSecurityRejectsAnonymousChanges verifies every security mutation redirects before effects.
func (fixture LibraryAPIFixture) AccountSecurityRejectsAnonymousChanges(t *testing.T) {
	handler := fixture.NewHandler("", t.TempDir(), true)
	for _, path := range []string{"/account/mfa/setup", "/account/mfa/enable", "/account/mfa/disable", "/account/oidc/unlink", "/account/saml/unlink"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup" {
			t.Fatalf("anonymous POST %s = %d", path, response.Code)
		}
	}
}

// PasskeyInventoryRejectsMalformedRemoval verifies strict identifiers and no side effects.
func (fixture LibraryAPIFixture) PasskeyInventoryRejectsMalformedRemoval(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	before := APICall(t, handler, token, http.MethodGet, "/api/v1/passkeys", nil)
	if before.Code != http.StatusOK || !strings.Contains(before.Body.String(), `"passkeys":[]`) {
		t.Fatalf("passkey inventory = %d %q", before.Code, before.Body.String())
	}
	invalid := APICall(t, handler, token, http.MethodDelete, "/api/v1/passkeys/not-a-fingerprint", nil)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("malformed removal = %d %q", invalid.Code, invalid.Body.String())
	}
	after := APICall(t, handler, token, http.MethodGet, "/api/v1/passkeys", nil)
	if after.Code != http.StatusOK || after.Body.String() != before.Body.String() {
		t.Fatalf("malformed removal changed inventory: before=%q after=%q", before.Body.String(), after.Body.String())
	}
}
