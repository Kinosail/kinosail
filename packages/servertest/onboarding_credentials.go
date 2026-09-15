package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// OnboardingFixture binds real application handlers and keeps app-specific setup copy explicit.
type OnboardingFixture struct {
	Library                 LibraryAPIFixture
	Web                     AuthCookieRequest
	TOTP                    func(*testing.T, string, time.Time) string
	Introduction, PlanLabel string
}

// OwnerSetupContinuesToConnectionOnboarding preserves the original real-handler onboarding continuation scenario.
func (fixture OnboardingFixture) OwnerSetupContinuesToConnectionOnboarding(t *testing.T) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	setupPage := APICall(t, handler, "", http.MethodGet, "/setup", nil)
	AssertAPIBody(t, setupPage, http.StatusOK, "Make this server yours.", fixture.Introduction, "Your Owner account", fixture.PlanLabel, "Nothing leaves home.", `autocomplete="new-password"`)
	setupForm := url.Values{"name": {"Owner"}, "password": {"owner-password"}, "totp": {"true"}}
	setupRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader(setupForm.Encode()))
	setupRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, setupRequest)
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(setup.Body.String())
	if setup.Code != http.StatusOK || len(secret) != 2 || !strings.Contains(setup.Body.String(), `name="next" value="/onboarding/connection"`) {
		t.Fatalf("Owner setup continuation = %d %q", setup.Code, setup.Body.String())
	}
	confirmForm := url.Values{"code": {fixture.TOTP(t, secret[1], time.Now())}, "next": {"/onboarding/connection"}}
	confirmRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa/enable", strings.NewReader(confirmForm.Encode()))
	confirmRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirmRequest.AddCookie(setup.Result().Cookies()[0])
	confirmed := httptest.NewRecorder()
	handler.ServeHTTP(confirmed, confirmRequest)
	if confirmed.Code != http.StatusSeeOther || confirmed.Header().Get("Location") != "/onboarding/connection" {
		t.Fatalf("confirmed setup = %d, location = %q", confirmed.Code, confirmed.Header().Get("Location"))
	}
	passkeys := APICall(t, handler, "", http.MethodGet, "/static/passkeys.js", nil)
	AssertAPIBody(t, passkeys, http.StatusOK, `location.replace("/onboarding/connection")`)
}

// OwnerAuthenticatorChoiceRetainsConnectionOnboarding preserves the original real-handler onboarding continuation scenario.
func (fixture OnboardingFixture) OwnerAuthenticatorChoiceRetainsConnectionOnboarding(t *testing.T) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	setup := fixture.Library.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	prompt := fixture.Web(t, handler, http.MethodGet, "/account?setup=1", "", setup)
	AssertAPIBody(t, prompt, http.StatusOK, `name="next" value="/onboarding/connection"`)
	form := url.Values{"next": {"/onboarding/connection"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa/setup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(setup)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	AssertAPIBody(t, response, http.StatusOK, `name="next" value="/onboarding/connection"`)
}
