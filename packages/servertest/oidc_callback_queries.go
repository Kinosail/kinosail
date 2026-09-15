package servertest

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// AssertOIDCCallbackRejectsInvalidQueriesBeforeExchange preserves pending transactions after malformed callbacks.
func AssertOIDCCallbackRejectsInvalidQueriesBeforeExchange(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	handler, _ := provider.handler(t, fixture, "")
	for name, malformed := range map[string]func(url.Values){
		"duplicate code":      func(values url.Values) { values["code"] = []string{"approved", "other"} },
		"duplicate state":     func(values url.Values) { values["state"] = []string{values.Get("state"), "other"} },
		"duplicate extension": func(values url.Values) { values["session_state"] = []string{"one", "two"} },
		"missing code":        func(values url.Values) { values.Del("code") },
		"oversized code":      func(values url.Values) { values.Set("code", strings.Repeat("x", 4097)) },
		"oversized extension": func(values url.Values) { values.Set("session_state", strings.Repeat("x", 4097)) },
	} {
		t.Run(name, func(t *testing.T) {
			authorization, cookie := provider.start(t, handler)
			values := url.Values{"code": {"approved"}, "state": {authorization.Query().Get("state")}}
			malformed(values)
			before := provider.exchanges.Load()
			response := callbackOIDC(t, handler, values.Encode(), cookie)
			if response.Code != http.StatusBadRequest || provider.exchanges.Load() != before {
				t.Fatalf("invalid callback = %d exchanges=%d body=%q", response.Code, provider.exchanges.Load()-before, response.Body.String())
			}
			valid := callbackOIDC(t, handler, "code=approved&state="+url.QueryEscape(authorization.Query().Get("state")), cookie)
			if valid.Code != http.StatusUnauthorized || provider.exchanges.Load() != before+1 {
				t.Fatalf("valid retry = %d exchanges=%d body=%q", valid.Code, provider.exchanges.Load()-before, valid.Body.String())
			}
		})
	}
}

// AssertOIDCCallbackAcceptsBoundedProviderExtensions preserves accepted provider response extensions.
func AssertOIDCCallbackAcceptsBoundedProviderExtensions(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	handler, _ := provider.handler(t, fixture, "")
	authorization, cookie := provider.start(t, handler)
	values := url.Values{"code": {"approved"}, "state": {authorization.Query().Get("state")}, "iss": {provider.issuer}, "session_state": {"provider-session"}, "organization": {"family"}}
	callback := callbackOIDC(t, handler, values.Encode(), cookie)
	if callback.Code != http.StatusUnauthorized || provider.exchanges.Load() != 1 {
		t.Fatalf("extension callback = %d exchanges=%d body=%q", callback.Code, provider.exchanges.Load(), callback.Body.String())
	}
}

// AssertOIDCCallbackValidatesAuthorizationResponseIssuer rejects a mismatched issuer without consuming the valid transaction.
func AssertOIDCCallbackValidatesAuthorizationResponseIssuer(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	provider := newOIDCCallbackProvider(t, true)
	handler, _ := provider.handler(t, fixture, "")
	authorization, cookie := provider.start(t, handler)
	state := authorization.Query().Get("state")
	mismatched := callbackOIDC(t, handler, "code=approved&iss=https%3A%2F%2Fattacker.example&state="+url.QueryEscape(state), cookie)
	if mismatched.Code != http.StatusBadRequest || provider.exchanges.Load() != 0 {
		t.Fatalf("mismatched callback issuer = %d exchanges=%d body=%q", mismatched.Code, provider.exchanges.Load(), mismatched.Body.String())
	}
	matching := callbackOIDC(t, handler, "code=approved&iss="+url.QueryEscape(provider.issuer)+"&state="+url.QueryEscape(state), cookie)
	if matching.Code != http.StatusUnauthorized || provider.exchanges.Load() != 1 {
		t.Fatalf("matching callback issuer = %d exchanges=%d body=%q", matching.Code, provider.exchanges.Load(), matching.Body.String())
	}
}
