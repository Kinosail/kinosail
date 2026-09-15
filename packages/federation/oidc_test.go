package federation

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestOIDCConfigurationAndValueValidation(t *testing.T) {
	t.Parallel()
	valid := OIDCConfig{Issuer: "https://identity.example", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://media.example/login/oidc/callback"}
	if !NewOIDC(valid).Configured() || (*OIDC)(nil).Configured() {
		t.Fatal("valid OIDC configuration was not distinguished from nil")
	}
	for name, mutate := range map[string]func(*OIDCConfig){
		"missing client":   func(config *OIDCConfig) { config.ClientID = "" },
		"large issuer":     func(config *OIDCConfig) { config.Issuer = "https://identity.example/" + strings.Repeat("x", 2049) },
		"large redirect":   func(config *OIDCConfig) { config.RedirectURL = "https://media.example/" + strings.Repeat("x", 2049) },
		"large client":     func(config *OIDCConfig) { config.ClientID = strings.Repeat("x", 513) },
		"large secret":     func(config *OIDCConfig) { config.ClientSecret = strings.Repeat("x", 4097) },
		"large claim":      func(config *OIDCConfig) { config.IdentityClaim = strings.Repeat("x", 257) },
		"malformed client": func(config *OIDCConfig) { config.ClientID = string([]byte{0xff}) },
		"query issuer":     func(config *OIDCConfig) { config.Issuer += "?unexpected=true" },
		"remote http":      func(config *OIDCConfig) { config.RedirectURL = "http://media.example/callback" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if NewOIDC(candidate).Configured() {
				t.Fatal("invalid OIDC configuration was accepted")
			}
		})
	}
	for raw, want := range map[string]bool{
		"https://identity.example/path": true,
		"http://localhost/path":         true,
		"http://127.0.0.1/path":         true,
		"http://identity.example/path":  false,
		"ftp://localhost/path":          false,
		"https://user@identity.example": false,
		"https://identity.example?q=1":  false,
		"https://identity.example/#x":   false,
	} {
		parsed, _ := url.Parse(raw)
		if TrustedURL(parsed) != want {
			t.Fatalf("TrustedURL(%q) = %v", raw, !want)
		}
	}
	if TrustedURL(nil) {
		t.Fatal("nil URL was trusted")
	}
	for subject, want := range map[string]bool{"subject": true, "": false, strings.Repeat("x", 256): false, "bad\nsubject": false, string([]byte{0xff}): false} {
		if ValidSubject(subject) != want {
			t.Fatalf("ValidSubject(%q) = %v", subject, !want)
		}
	}
}

func TestOIDCCallbackParserRejectsAmbiguousInput(t *testing.T) {
	t.Parallel()
	valid := "code=approved&state=state"
	for name, raw := range map[string]string{
		"large query":         strings.Repeat("x", 16<<10+1),
		"bad encoding":        "%zz",
		"missing state":       "code=approved",
		"duplicate state":     "code=approved&state=one&state=two",
		"duplicate code":      "code=one&code=two&state=state",
		"control extension":   "code=approved&state=state&extra=%0A",
		"unknown bad name":    "code=approved&state=state&bad%5B%5D=x",
		"code and error":      valid + "&error=denied",
		"bad error":           "error=bad%5Cvalue&state=state",
		"large error details": "error=denied&error_description=" + strings.Repeat("x", 2049) + "&state=state",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, ok := parseOIDCCallbackQuery(raw); ok {
				t.Fatal("invalid callback was accepted")
			}
		})
	}
	code, issuer, denied, ok := parseOIDCCallbackQuery(valid + "&iss=https%3A%2F%2Fidentity.example&session_state=one")
	if !ok || denied || code != "approved" || issuer != "https://identity.example" {
		t.Fatalf("valid callback = %q %q %v %v", code, issuer, denied, ok)
	}
	_, _, denied, ok = parseOIDCCallbackQuery("error=access_denied&error_description=canceled&error_uri=https%3A%2F%2Fidentity.example%2Ferror&state=state")
	if !ok || !denied {
		t.Fatal("valid provider error was rejected")
	}
}

func TestOIDCStateCookiePolicy(t *testing.T) {
	t.Parallel()
	secure := StateCookie("state", true)
	cleared := StateCookie("", false)
	if secure.Name != OIDCStateCookieName || !secure.Secure {
		t.Fatalf("state cookies = %#v %#v", secure, cleared)
	}
	if secure.MaxAge != 600 || cleared.MaxAge != -1 {
		t.Fatalf("state cookie ages = %d %d", secure.MaxAge, cleared.MaxAge)
	}
}

func TestOIDCMFAChallengeLifecycle(t *testing.T) {
	t.Parallel()
	flow := NewOIDC(OIDCConfig{})
	challenge, err := flow.BeginMFA("viewer")
	profileID, found := flow.MFAProfile(challenge)
	if err != nil || !found {
		t.Fatalf("MFA challenge = %q %v %v", profileID, found, err)
	}
	if profileID != "viewer" {
		t.Fatalf("MFA profile = %q", profileID)
	}
	consumedID, consumed := flow.ConsumeMFA(challenge)
	if !consumed {
		t.Fatalf("consumed MFA challenge = %q %v", consumedID, consumed)
	}
	if consumedID != "viewer" {
		t.Fatalf("consumed MFA profile = %q", consumedID)
	}
	if _, found = flow.MFAProfile(challenge); found {
		t.Fatal("MFA challenge replay was accepted")
	}
}

func TestOIDCMFAStoreIsBounded(t *testing.T) {
	t.Parallel()
	flow := NewOIDC(OIDCConfig{})
	for index := range maxPendingStates {
		flow.mfa[string(rune(index+1))] = mfaChallenge{expires: time.Now().Add(time.Minute)}
	}
	if _, err := flow.BeginMFA("viewer"); !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("MFA pending limit = %v", err)
	}
	flow.mfa["expired"] = mfaChallenge{expires: time.Now().Add(-time.Minute)}
	delete(flow.mfa, string(rune(1)))
	if _, err := flow.BeginMFA("viewer"); err != nil {
		t.Fatalf("expired MFA challenge was not pruned: %v", err)
	}
}

func TestOIDCFlowValidatesBeforeEffectsAndCompletesOnce(t *testing.T) { //nolint:funlen // One provider proves every transaction effect boundary.
	t.Parallel()
	flow, exchanges, setNonce := newOIDCTestFlow(t)
	state, nonce := mustBeginOIDC(t, flow)
	setNonce(nonce)
	assertOIDCCallbackError(t, flow, "code=approved&code=other&state="+url.QueryEscape(state), state, ErrInvalidCallback)
	assertExchanges(t, exchanges, 0)
	result, err := flow.Complete(t.Context(), "code=approved&state="+url.QueryEscape(state), state, true)
	assertOIDCCallback(t, result, err)
	assertExchanges(t, exchanges, 1)
	assertOIDCCallbackError(t, flow, "code=approved&state="+url.QueryEscape(state), state, ErrInvalidState)
	assertExchanges(t, exchanges, 1)
}

func mustBeginOIDC(t *testing.T, flow *OIDC) (string, string) {
	t.Helper()
	location, state, err := flow.Begin(t.Context(), "viewer")
	authorization, parseErr := url.Parse(location)
	if err != nil || parseErr != nil {
		t.Fatalf("OIDC begin = %q %q %v %v", location, state, err, parseErr)
	}
	if state == "" || authorization.Query().Get("scope") != "openid" {
		t.Fatalf("OIDC authorization = %q state=%q", location, state)
	}
	return state, authorization.Query().Get("nonce")
}

func assertOIDCCallbackError(t *testing.T, flow *OIDC, query, state string, want error) {
	t.Helper()
	if _, err := flow.Complete(t.Context(), query, state, true); !errors.Is(err, want) {
		t.Fatalf("OIDC callback error = %v want %v", err, want)
	}
}

func assertOIDCCallback(t *testing.T, result OIDCCallback, err error) {
	t.Helper()
	if err != nil || result.ProfileID != "viewer" {
		t.Fatalf("OIDC complete = %#v %v", result, err)
	}
	if result.Identity.Subject != "subject-1" || !result.ClearCookie {
		t.Fatalf("OIDC identity = %#v", result)
	}
}

func assertExchanges(t *testing.T, exchanges *atomic.Int32, want int32) {
	t.Helper()
	if exchanges.Load() != want {
		t.Fatalf("OIDC exchanges = %d want %d", exchanges.Load(), want)
	}
}

func newOIDCTestFlow(t *testing.T) (*OIDC, *atomic.Int32, func(string)) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, nonce := "", ""
	var exchanges atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			exchanges.Add(1)
		}
		serveOIDC(t, writer, request, issuer, nonce, privateKey)
	}))
	t.Cleanup(provider.Close)
	issuer = provider.URL
	flow := NewOIDC(OIDCConfig{Issuer: issuer, ClientID: "kinosail", ClientSecret: "secret", RedirectURL: "http://localhost/login/oidc/callback"})
	return flow, &exchanges, func(value string) { nonce = value }
}

func TestOIDCFlowRejectsProviderAndCallbackFailures(t *testing.T) { //nolint:funlen // Table cases preserve each public error mode.
	t.Parallel()
	if _, _, err := NewOIDC(OIDCConfig{}).Begin(t.Context(), ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured begin = %v", err)
	}
	issuer := ""
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": "http://identity.example/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(provider.Close)
	issuer = provider.URL
	untrusted := NewOIDC(OIDCConfig{Issuer: issuer, ClientID: "client", ClientSecret: "secret", RedirectURL: "http://localhost/callback"})
	if _, _, err := untrusted.Begin(t.Context(), ""); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("untrusted provider = %v", err)
	}
	flow, state := oidcFlowWithPending("https://identity.example")
	if _, err := flow.Complete(t.Context(), "code=approved&iss=https%3A%2F%2Fother.example&state="+state, state, true); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("issuer mismatch = %v", err)
	}
	if _, err := flow.Complete(t.Context(), "error=access_denied&state="+state, state, true); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("provider denial = %v", err)
	}
	flow, state = oidcFlowWithPending("https://identity.example")
	if _, err := flow.Complete(t.Context(), "code=approved&state="+state, "other", true); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("state mismatch = %v", err)
	}
	flow, state = oidcFlowWithPending("https://identity.example")
	if _, err := flow.Complete(t.Context(), "code=approved&state="+state, state, true); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unavailable provider = %v", err)
	}
}

func oidcFlowWithPending(issuer string) (*OIDC, string) {
	flow := NewOIDC(OIDCConfig{Issuer: issuer, ClientID: "client", ClientSecret: "secret", RedirectURL: "https://media.example/callback"})
	state := "state"
	flow.pending[tokenKey(state)] = oidcTransaction{nonce: "nonce", verifier: "verifier", expires: time.Now().Add(time.Minute)}
	return flow, state
}

func serveOIDC(t *testing.T, writer http.ResponseWriter, request *http.Request, issuer, nonce string, privateKey *rsa.PrivateKey) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(writer).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
	case "/keys":
		_ = json.NewEncoder(writer).Encode(map[string]any{"keys": []jose.JSONWebKey{{Key: &privateKey.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
	case "/token":
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, new(jose.SignerOptions).WithType("JWT").WithHeader("kid", "test"))
		if err != nil {
			t.Fatal(err)
		}
		idToken, err := jwt.Signed(signer).Claims(map[string]any{"iss": issuer, "sub": "subject-1", "aud": "kinosail", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": nonce}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "access", "token_type": "Bearer", "id_token": idToken})
	default:
		http.NotFound(writer, request)
	}
}
