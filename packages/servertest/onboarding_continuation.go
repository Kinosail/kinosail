package servertest

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// OwnerCanRerunOnboardingFromSettings preserves the original real-handler onboarding continuation scenario.
func (fixture OnboardingFixture) OwnerCanRerunOnboardingFromSettings(t *testing.T) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	owner := fixture.Library.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")

	settings := fixture.Web(t, handler, http.MethodGet, "/settings", "", owner)
	AssertAPIBody(t, settings, http.StatusOK, `id="onboarding"`, "Setup guide", `action="/settings/onboarding"`, "Open setup guide")
	trigger := fixture.Web(t, handler, http.MethodPost, "/settings/onboarding", "", owner)
	if trigger.Code != http.StatusSeeOther || trigger.Header().Get("Location") != "/onboarding" {
		t.Fatalf("retrigger onboarding = %d, location = %q", trigger.Code, trigger.Header().Get("Location"))
	}
	start := fixture.Web(t, handler, http.MethodGet, "/onboarding", "", owner)
	if start.Code != http.StatusSeeOther || start.Header().Get("Location") != "/onboarding/connection" {
		t.Fatalf("start onboarding = %d, location = %q", start.Code, start.Header().Get("Location"))
	}
	finish := fixture.Web(t, handler, http.MethodGet, "/onboarding/finish", "", owner)
	if finish.Code != http.StatusSeeOther || finish.Header().Get("Location") != "/" {
		t.Fatalf("finish onboarding = %d, location = %q", finish.Code, finish.Header().Get("Location"))
	}
	settings = fixture.Web(t, handler, http.MethodGet, "/settings", "", owner)
	AssertAPIBody(t, settings, http.StatusOK, "Revisit your Server setup choices whenever you like.")
}

// OwnerFirstCredentialLoginOpensOnboarding preserves the original real-handler onboarding continuation scenario.
func (fixture OnboardingFixture) OwnerFirstCredentialLoginOpensOnboarding(t *testing.T) {
	handler := fixture.Library.NewHandler("", t.TempDir(), true)
	owner, secret := fixture.setupOwnerForLogin(t, handler)

	login := WebFormCall(t, handler, "", "/login", url.Values{"name": {"Owner"}, "password": {"owner-password"}, "code": {fixture.TOTP(t, secret, time.Now())}})
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/account?passkey=offer&next=%2Fonboarding" {
		t.Fatalf("first credential login = %d, location = %q", login.Code, login.Header().Get("Location"))
	}
	finish := fixture.Web(t, handler, http.MethodGet, "/onboarding/finish", "", owner)
	if finish.Code != http.StatusSeeOther {
		t.Fatalf("finish onboarding = %d, location = %q", finish.Code, finish.Header().Get("Location"))
	}
	login = WebFormCall(t, handler, "", "/login", url.Values{"name": {"Owner"}, "password": {"owner-password"}, "code": {fixture.TOTP(t, secret, time.Now())}})
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/account?passkey=offer&next=%2F" {
		t.Fatalf("later credential login = %d, location = %q", login.Code, login.Header().Get("Location"))
	}
}

func (fixture OnboardingFixture) setupOwnerForLogin(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	setup := WebFormCall(t, handler, "", "/setup", url.Values{"name": {"Owner"}, "password": {"owner-password"}, "totp": {"true"}})
	if setup.Code != http.StatusOK || len(setup.Result().Cookies()) != 1 {
		t.Fatalf("setup owner = %d, cookies = %v", setup.Code, setup.Result().Cookies())
	}
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(setup.Body.String())
	if len(secret) != 2 {
		t.Fatalf("setup factor = %d %q", setup.Code, setup.Body.String())
	}
	owner := setup.Result().Cookies()[0]
	confirmed := fixture.Web(t, handler, http.MethodPost, "/account/mfa/enable", "code="+fixture.TOTP(t, secret[1], time.Now()), owner)
	if confirmed.Code != http.StatusSeeOther {
		t.Fatalf("confirm setup factor = %d %q", confirmed.Code, confirmed.Body.String())
	}
	return owner, secret[1]
}
