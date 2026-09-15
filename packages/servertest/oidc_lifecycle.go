package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func assertUnlinkedOIDCIdentity(t *testing.T, handler http.Handler, nonce *string) {
	t.Helper()
	// A valid provider identity is rejected until a local Viewer explicitly links it.
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
	location := MustOIDCAuthorization(t, start)
	*nonce = location.Query().Get("nonce")
	stateCookie := OIDCStateCookie(t, start)
	callbackPath := "/login/oidc/callback?code=approved&state=" + url.QueryEscape(location.Query().Get("state"))
	callback := httptest.NewRecorder()
	handler.ServeHTTP(callback, OIDCCallbackRequest(t, callbackPath, nil))
	if callback.Code != http.StatusBadRequest || len(callback.Result().Cookies()) != 0 {
		t.Fatalf("cross-browser callback = %d cookies=%v body=%q", callback.Code, callback.Result().Cookies(), callback.Body.String())
	}
	validCallback := httptest.NewRecorder()
	validRequest := OIDCCallbackRequest(t, callbackPath, stateCookie)
	handler.ServeHTTP(validCallback, validRequest)
	if validCallback.Code != http.StatusUnauthorized {
		t.Fatalf("unlinked valid callback = %d cookies=%v body=%q", validCallback.Code, validCallback.Result().Cookies(), validCallback.Body.String())
	}
}

func assertOIDCProfileLink(t *testing.T, fixture OIDCFixture, handler http.Handler, dataDir, issuer string, nonce *string, owner *http.Cookie) []byte {
	t.Helper()
	// The already-authenticated Viewer initiates linking; no profile is provisioned.
	link := httptest.NewRecorder()
	linkRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me/oidc/link", nil)
	linkRequest.AddCookie(owner)
	handler.ServeHTTP(link, linkRequest)
	location := MustOIDCAuthorization(t, link)
	*nonce = location.Query().Get("nonce")
	stateCookie := OIDCStateCookie(t, link)
	callbackPath := "/login/oidc/callback?code=approved&state=" + url.QueryEscape(location.Query().Get("state"))
	linked := httptest.NewRecorder()
	handler.ServeHTTP(linked, OIDCCallbackRequest(t, callbackPath, stateCookie))
	if linked.Code != http.StatusSeeOther || linked.Header().Get("Location") != "/account" {
		t.Fatalf("link callback = %d %q", linked.Code, linked.Body.String())
	}
	profiles := fixture.StoredState(t, dataDir, "profiles.json")
	if strings.Count(string(profiles), `"id"`) != 1 || !strings.Contains(string(profiles), `"oidcIssuer":"`+issuer+`"`) || !strings.Contains(string(profiles), `"oidcSubject":"subject-1"`) {
		t.Fatalf("linked profiles = %q", profiles)
	}

	return profiles
}

func assertLinkedOIDCSessions(t *testing.T, fixture OIDCFixture, handler http.Handler, nonce *string, profiles []byte, secret string, owner *http.Cookie) {
	t.Helper()
	// The linked identity can now create a session for that exact local Profile.
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
	location := MustOIDCAuthorization(t, start)
	*nonce = location.Query().Get("nonce")
	stateCookie := OIDCStateCookie(t, start)
	callbackPath := "/login/oidc/callback?code=approved&state=" + url.QueryEscape(location.Query().Get("state"))
	callback := httptest.NewRecorder()
	handler.ServeHTTP(callback, OIDCCallbackRequest(t, callbackPath, stateCookie))
	linkedMFA, _ := url.Parse(callback.Header().Get("Location"))
	if callback.Code != http.StatusSeeOther || linkedMFA.Path != "/login/mfa" || linkedMFA.Query().Get("challenge") == "" {
		t.Fatalf("linked SSO challenge = %d %q", callback.Code, callback.Header().Get("Location"))
	}
	linkedFinishRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/mfa", strings.NewReader(url.Values{"challenge": {linkedMFA.Query().Get("challenge")}, "code": {fixture.TOTP(t, secret, time.Now())}}.Encode()))
	linkedFinishRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	linkedFinish := httptest.NewRecorder()
	handler.ServeHTTP(linkedFinish, linkedFinishRequest)
	mustOIDCCallback(t, linkedFinish, profiles, nil)
	assertOIDCRepeatSession(t, fixture, handler, nonce, secret, owner, callbackPath)
}

func assertOIDCRepeatSession(t *testing.T, fixture OIDCFixture, handler http.Handler, nonce *string, secret string, owner *http.Cookie, callbackPath string) {
	t.Helper()
	settingsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	settingsRequest.AddCookie(owner)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, settingsRequest)
	if !strings.Contains(settings.Body.String(), "OpenID Connect") || !strings.Contains(settings.Body.String(), "Enabled") {
		t.Fatalf("SSO settings = %d %q", settings.Code, settings.Body.String())
	}
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
	location := MustOIDCAuthorization(t, start)
	*nonce = location.Query().Get("nonce")
	stateCookie := OIDCStateCookie(t, start)
	mfaCallback := httptest.NewRecorder()
	handler.ServeHTTP(mfaCallback, OIDCCallbackRequest(t, "/login/oidc/callback?code=approved&state="+url.QueryEscape(location.Query().Get("state")), stateCookie))
	mfaLocation, _ := url.Parse(mfaCallback.Header().Get("Location"))
	if mfaCallback.Code != http.StatusSeeOther || mfaLocation.Path != "/login/mfa" || mfaLocation.Query().Get("challenge") == "" {
		t.Fatalf("SSO MFA challenge = %d %q cookies=%v", mfaCallback.Code, mfaCallback.Header().Get("Location"), mfaCallback.Result().Cookies())
	}
	mfaRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/mfa", strings.NewReader(url.Values{"challenge": {mfaLocation.Query().Get("challenge")}, "code": {fixture.TOTP(t, secret, time.Now())}}.Encode()))
	mfaRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mfaFinish := httptest.NewRecorder()
	handler.ServeHTTP(mfaFinish, mfaRequest)
	if mfaFinish.Code != http.StatusSeeOther || len(mfaFinish.Result().Cookies()) != 1 {
		t.Fatalf("SSO MFA finish = %d cookies=%v %q", mfaFinish.Code, mfaFinish.Result().Cookies(), mfaFinish.Body.String())
	}
	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, httptest.NewRequestWithContext(t.Context(), http.MethodGet, callbackPath, nil))
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replayed callback = %d %q", replay.Code, replay.Body.String())
	}
}
