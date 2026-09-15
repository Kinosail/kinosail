package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// AssertOIDCRequestsOnlyTheIdentityScopeItUses checks the minimal identity scope.
func AssertOIDCRequestsOnlyTheIdentityScopeItUses(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, false)
	handler, _ := provider.handler(t, fixture, "")
	authorization, _ := provider.start(t, handler)
	if scope := authorization.Query().Get("scope"); scope != "openid" {
		t.Fatalf("authorization scope = %q", scope)
	}
}

// AssertOIDCRejectsMissingConfiguredIdentityClaimWithoutLinking checks durable identity state after rejected claims.
func AssertOIDCRejectsMissingConfiguredIdentityClaimWithoutLinking(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	handler, dataDir := provider.handler(t, fixture, "missing")
	if setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password"}); setup.Code != http.StatusCreated {
		t.Fatalf("OIDC missing-claim setup = %d %q", setup.Code, setup.Body.String())
	}
	authorization, cookie := provider.start(t, handler)
	callback := callbackOIDC(t, handler, "code=approved&state="+url.QueryEscape(authorization.Query().Get("state")), cookie)
	if callback.Code != http.StatusUnauthorized || strings.Contains(string(fixture.StoredState(t, dataDir, "profiles.json")), `"oidcSubject"`) {
		t.Fatalf("missing identity claim callback=%d profiles=%q body=%q", callback.Code, fixture.StoredState(t, dataDir, "profiles.json"), callback.Body.String())
	}
}

// AssertOIDCSupportsClientSecretPostProviders checks the existing token authentication fallback.
func AssertOIDCSupportsClientSecretPostProviders(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	provider.postOnly = true
	handler, _ := provider.handler(t, fixture, "")
	authorization, cookie := provider.start(t, handler)
	callback := callbackOIDC(t, handler, "code=approved&state="+url.QueryEscape(authorization.Query().Get("state")), cookie)
	if callback.Code != http.StatusUnauthorized || provider.basicAttempts.Load() != 1 || provider.postAttempts.Load() != 1 {
		t.Fatalf("client_secret_post callback=%d basic=%d post=%d body=%q", callback.Code, provider.basicAttempts.Load(), provider.postAttempts.Load(), callback.Body.String())
	}
}

// AssertOIDCCallbackHandlesProviderErrorWithoutExchange checks privacy, cookie clearing, and transaction consumption.
func AssertOIDCCallbackHandlesProviderErrorWithoutExchange(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	handler, _ := provider.handler(t, fixture, "")
	authorization, cookie := provider.start(t, handler)
	state := url.QueryEscape(authorization.Query().Get("state"))
	callback := callbackOIDC(t, handler, "error=access_denied&error_description=User+canceled&state="+state, cookie)
	assertOIDCProviderError(t, callback, provider.exchanges.Load())
	replay := callbackOIDC(t, handler, "code=approved&state="+state, cookie)
	if replay.Code != http.StatusBadRequest || provider.exchanges.Load() != 0 {
		t.Fatalf("provider error replay = %d exchanges=%d body=%q", replay.Code, provider.exchanges.Load(), replay.Body.String())
	}
}

func assertOIDCProviderError(t *testing.T, callback *httptest.ResponseRecorder, exchanges int32) {
	t.Helper()
	if callback.Code != http.StatusBadRequest || exchanges != 0 || strings.Contains(callback.Body.String(), "User canceled") {
		t.Fatalf("provider error callback = %d exchanges=%d body=%q", callback.Code, exchanges, callback.Body.String())
	}
	if cookies := callback.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "kinosail_oidc_state" || cookies[0].MaxAge >= 0 {
		t.Fatalf("provider error cookies = %v", cookies)
	}
}
