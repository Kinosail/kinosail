package servertest

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// OIDCStateCookie provides the shared OIDC protocol fixture.
func OIDCStateCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "kinosail_oidc_state" && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatal("OIDC state cookie was not set")
	return nil
}

// OIDCCallbackRequest provides the shared OIDC protocol fixture.
func OIDCCallbackRequest(t *testing.T, path string, state *http.Cookie) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if state != nil {
		request.AddCookie(state)
	}
	return request
}

func mustOIDCLogin(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(response.Body.String(), `type="password"`) || !strings.Contains(response.Body.String(), "Sign in with SSO") {
		t.Fatalf("login options = %q", response.Body.String())
	}
}

// MustOIDCAuthorization provides the shared OIDC protocol fixture.
func MustOIDCAuthorization(t *testing.T, response *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil || location.Query().Get("state") == "" || location.Query().Get("nonce") == "" || location.Query().Get("code_challenge") == "" {
		t.Fatalf("authorization redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
	return location
}

func mustOIDCCallback(t *testing.T, response *httptest.ResponseRecorder, profiles []byte, err error) {
	t.Helper()
	if response.Code != http.StatusSeeOther || len(response.Result().Cookies()) != 1 || err != nil || !strings.Contains(string(profiles), `"oidcSubject":"subject-1"`) {
		t.Fatalf("callback = %d cookies=%v profiles=%q err=%v body=%q", response.Code, response.Result().Cookies(), profiles, err, response.Body.String())
	}
}

// FakeOIDC provides the shared OIDC protocol fixture.
func FakeOIDC(t *testing.T, writer http.ResponseWriter, request *http.Request, issuer, nonce string, private *rsa.PrivateKey) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(writer).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}, "token_endpoint_auth_methods_supported": []string{"client_secret_basic"}})
	case "/keys":
		_ = json.NewEncoder(writer).Encode(map[string]any{"keys": []jose.JSONWebKey{{Key: &private.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
	case "/token":
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: private}, new(jose.SignerOptions).WithType("JWT").WithHeader("kid", "test"))
		if err != nil {
			t.Fatal(err)
		}
		idToken, err := jwt.Signed(signer).Claims(map[string]any{"iss": issuer, "sub": "subject-1", "oid": "directory-1", "aud": "kinosail", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "name": "Owner", "email": "jane@example.com", "email_verified": true}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"access_token": "access", "token_type": "Bearer", "id_token": idToken})
	default:
		http.NotFound(writer, request)
	}
}
