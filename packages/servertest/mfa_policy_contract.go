package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// AssertOwnerCanRequireMFAForEveryProfile preserves the original MFA enrollment and policy assertions.
func AssertOwnerCanRequireMFAForEveryProfile(t *testing.T, fixture MFAPolicyFixture) {
	data := t.TempDir()
	handler := fixture.Library.NewHandler("", data, true)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	MustJSON(t, setup, &owner)
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, owner.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	created := APICall(t, handler, owner.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create Viewer = %d %q", created.Code, created.Body.String())
	}
	settingsPage := APICall(t, handler, owner.Token, http.MethodGet, "/settings", nil)
	AssertAPIBody(t, settingsPage, http.StatusOK, `action="/settings/mfa"`, `name="required"`, `name="required" value="true" checked`, `TOTP: Enabled`)
	required := APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/settings/mfa", map[string]any{"required": true})
	AssertAPIBody(t, required, http.StatusOK, `"required":true`, `"sessionsRevoked":false`)

	ownerLogin := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": TestTOTP(t, owner.TOTP.Secret, time.Now())})
	var loggedInOwner struct{ Token string }
	MustJSON(t, ownerLogin, &loggedInOwner)
	AssertAPIBody(t, ownerLogin, http.StatusCreated, `"mfaEnrollmentRequired":false`)
	AssertAPIBody(t, APICall(t, handler, loggedInOwner.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"requireMfa":true`)
	AssertAPIBody(t, APICall(t, handler, loggedInOwner.Token, http.MethodGet, "/api/v1/profiles", nil), http.StatusOK, `"name":"Owner"`, `"mfaEnabled":true`)

	viewerLogin := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var viewer struct{ Token string }
	MustJSON(t, viewerLogin, &viewer)
	AssertAPIBody(t, viewerLogin, http.StatusCreated, `"mfaEnrollmentRequired":true`)
	handler = fixture.Library.NewHandler("", data, true)
	AssertAPIBody(t, APICall(t, handler, viewer.Token, http.MethodGet, "/api/v1/library", nil), http.StatusForbidden, `"mfaEnrollmentRequired":true`)

	webLogin := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Viewer&password=viewer-password"))
	webLogin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webResponse := httptest.NewRecorder()
	handler.ServeHTTP(webResponse, webLogin)
	if webResponse.Code != http.StatusSeeOther || webResponse.Header().Get("Location") != "/account?mfa=required" {
		t.Fatalf("required web login = %d %q", webResponse.Code, webResponse.Header().Get("Location"))
	}
	disabled := APICall(t, handler, loggedInOwner.Token, http.MethodPut, "/api/v1/settings/mfa", map[string]any{"required": false})
	AssertAPIBody(t, disabled, http.StatusOK, `"required":false`, `"sessionsRevoked":false`)
	if response := APICall(t, handler, viewer.Token, http.MethodGet, "/api/v1/library", nil); response.Code != http.StatusOK {
		t.Fatalf("Viewer after disabling MFA requirement = %d %q", response.Code, response.Body.String())
	}
}

// AssertOwnerCanRequireMFAThroughWeb preserves the original MFA enrollment and policy assertions.
func AssertOwnerCanRequireMFAThroughWeb(t *testing.T, fixture MFAPolicyFixture) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var result struct {
		Token string
		TOTP  struct{ Secret string }
	}
	MustJSON(t, setup, &result)
	AssertAPIBody(t, APICall(t, handler, result.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, result.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	owner := &http.Cookie{Name: "__Host-kinosail_session", Value: result.Token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	response := fixture.Web(t, handler, http.MethodPost, "/settings/mfa", "required=false", owner)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings#security" {
		t.Fatalf("web MFA requirement = %d %q", response.Code, response.Header().Get("Location"))
	}
	response = fixture.Web(t, handler, http.MethodPost, "/settings/mfa", "required=true", owner)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("web MFA re-enable = %d %q", response.Code, response.Header().Get("Location"))
	}
	login := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Owner&password=owner-password&code="+TestTOTP(t, result.TOTP.Secret, time.Now())))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	if loggedIn.Code != http.StatusSeeOther || loggedIn.Header().Get("Location") != "/account?passkey=offer&next=%2Fonboarding" {
		t.Fatalf("required Owner login = %d %q", loggedIn.Code, loggedIn.Header().Get("Location"))
	}
}
