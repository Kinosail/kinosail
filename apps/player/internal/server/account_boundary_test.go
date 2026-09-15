package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestViewerCanReachAccountSecurityThroughWebAndAPI(t *testing.T) { //nolint:cyclop // One boundary test keeps the web and API account workflow together.
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	home := requestWithCookie(t, handler, http.MethodGet, "/", "", cookie)
	if !strings.Contains(home.Body.String(), `href="/account">Owner</a>`) {
		t.Fatalf("home does not expose account security: %q", home.Body.String())
	}
	for _, path := range []string{"/account", "/account?setup=1"} {
		response := requestWithCookie(t, handler, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "passkey") {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	setup := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", cookie)
	if setup.Code != http.StatusOK || !strings.Contains(setup.Body.String(), "Recovery codes") {
		t.Fatalf("MFA setup = %d %q", setup.Code, setup.Body.String())
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/enable", "code=invalid", cookie); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid MFA enable = %d %q", response.Code, response.Body.String())
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/disable", "code=invalid", cookie); response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid MFA disable = %d %q", response.Code, response.Body.String())
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/account/oidc/unlink", "", cookie); response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/account" {
		t.Fatalf("web OIDC unlink = %d %q", response.Code, response.Body.String())
	}

	response := apiCall(t, handler, cookie.Value, http.MethodDelete, "/api/v1/me/oidc", nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("API OIDC unlink = %d %q", response.Code, response.Body.String())
	}
}

func TestAccountSecurityRejectsAnonymousChanges(t *testing.T) {
	libraryAPIFixture.AccountSecurityRejectsAnonymousChanges(t)
}

func TestPasskeyInventoryAPIRejectsMalformedRemovalWithoutSideEffects(t *testing.T) {
	libraryAPIFixture.PasskeyInventoryRejectsMalformedRemoval(t)
}
