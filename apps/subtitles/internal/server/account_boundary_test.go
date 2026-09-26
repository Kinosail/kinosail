package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
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
	if setup.Code != http.StatusOK || !strings.Contains(setup.Body.String(), "Recovery codes") || !strings.Contains(setup.Body.String(), "Save every recovery code before you continue") {
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

func TestViewerHeaderLinksToAccountWithoutOwnerActions(t *testing.T) {
	handler := server.New(server.Config{SubtitleApp: true, DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	viewer := addAndSignInViewer(t, handler, owner, "Sam", "viewer-password")
	account := requestWithCookie(t, handler, http.MethodGet, "/account", "", viewer)
	if account.Code != http.StatusOK {
		t.Fatalf("viewer account = %d", account.Code)
	}
	header := strings.SplitN(account.Body.String(), "</header>", 2)[0]
	if !strings.Contains(header, `href="/account"`) || strings.Contains(header, `href="/settings"`) || strings.Contains(header, `href="/supporter"`) {
		t.Fatalf("viewer header has the wrong actions: %q", header)
	}
}

func TestAccountSecurityRejectsAnonymousChanges(t *testing.T) {
	scenario := libraryAPIFixture.AccountSecurityRejectsAnonymousChanges
	scenario(t)
}

func TestPasskeyInventoryAPIRejectsMalformedRemovalWithoutSideEffects(t *testing.T) {
	scenario := libraryAPIFixture.PasskeyInventoryRejectsMalformedRemoval
	scenario(t)
}
