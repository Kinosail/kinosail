package servertest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// MFAPolicyFixture binds the existing real server and authenticated web helper.
type MFAPolicyFixture struct {
	Library LibraryAPIFixture
	Web     AuthCookieRequest
}

// AssertSingleOwnerMustEnrollOneAuthenticationFactor preserves the original MFA enrollment and policy assertions.
func AssertSingleOwnerMustEnrollOneAuthenticationFactor(t *testing.T, fixture MFAPolicyFixture) {
	t.Parallel()

	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password"})
	var owner struct{ Token string }
	MustJSON(t, setup, &owner)
	AssertAPIBody(t, setup, http.StatusCreated, `"mfaEnrollmentRequired":true`)
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodGet, "/api/v1/library", nil), http.StatusForbidden, `"mfaEnrollmentRequired":true`)

	enrollmentResponse := APICall(t, handler, owner.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var enrollment struct{ Secret string }
	MustJSON(t, enrollmentResponse, &enrollment)
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, enrollment.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	if response := APICall(t, handler, owner.Token, http.MethodGet, "/api/v1/library", nil); response.Code != http.StatusOK {
		t.Fatalf("secured Owner library = %d %q", response.Code, response.Body.String())
	}
}

// AssertOwnerCanChooseTOTPWhenSigningUpThroughWebAndAPI preserves both original signup scenarios.
func AssertOwnerCanChooseTOTPWhenSigningUpThroughWebAndAPI(t *testing.T, fixture MFAPolicyFixture) {
	t.Run("web", func(t *testing.T) { assertWebMFASignup(t, fixture) })
	t.Run("API", func(t *testing.T) { assertAPIMFASignup(t, fixture) })
}

func assertWebMFASignup(t *testing.T, fixture MFAPolicyFixture) {
	t.Helper()
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	page := APICall(t, handler, "", http.MethodGet, "/setup", nil)
	AssertAPIBody(t, page, http.StatusOK, `name="totp" value="true" checked`, "MFA - Add extra sign-in protection now")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password=owner-password&totp=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(response.Body.String())
	if response.Code != http.StatusOK || len(secret) != 2 || len(response.Result().Cookies()) != 1 || !strings.Contains(response.Body.String(), "Recovery codes") {
		t.Fatalf("TOTP web signup = %d cookies=%v %q", response.Code, response.Result().Cookies(), response.Body.String())
	}
	confirmed := fixture.Web(t, handler, http.MethodPost, "/account/mfa/enable", "code="+TestTOTP(t, secret[1], time.Now()), response.Result().Cookies()[0])
	if confirmed.Code != http.StatusSeeOther || confirmed.Header().Get("Location") != "/account" {
		t.Fatalf("confirm signup TOTP = %d %q", confirmed.Code, confirmed.Body.String())
	}
	AssertAPIBody(t, APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password"}), http.StatusUnauthorized, `"mfaRequired":true`)
}

func assertAPIMFASignup(t *testing.T, fixture MFAPolicyFixture) {
	t.Helper()
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Installer", "totp": true})
	var result struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret        string   `json:"secret"`
			RecoveryCodes []string `json:"recoveryCodes"`
		} `json:"totp"`
	}
	MustJSON(t, setup, &result)
	if setup.Code != http.StatusCreated || result.Token == "" || result.TOTP.Secret == "" || len(result.TOTP.RecoveryCodes) != 10 {
		t.Fatalf("TOTP API signup = %d %q", setup.Code, setup.Body.String())
	}
	confirmed := APICall(t, handler, result.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, result.TOTP.Secret, time.Now())})
	AssertAPIBody(t, confirmed, http.StatusOK, `"enabled":true`)
}
