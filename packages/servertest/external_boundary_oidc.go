package servertest

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/scim"
)

// AssertExternalOIDCBoundaries checks unsafe origins and fail-closed protocol transitions.
func AssertExternalOIDCBoundaries(t *testing.T, fixture OIDCFixture) {
	assertExternalOIDCOrigins(t, fixture)
	assertExternalOIDCTransitions(t, fixture)
}

func assertExternalOIDCOrigins(t *testing.T, fixture OIDCFixture) {
	t.Helper()
	for _, config := range []federation.OIDCConfig{
		{},
		{Issuer: "http://example.com", ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/callback"},
		{Issuer: "https://user@example.com", ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/callback"},
		{Issuer: "https://example.com?query=1", ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/callback"},
		{Issuer: "https://example.com", ClientID: "id", ClientSecret: "secret", RedirectURL: "ftp://localhost/callback"},
	} {
		handler := fixture.NewHandler(t.TempDir(), config, scim.Config{})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("OIDC config %#v = %d", config, response.Code)
		}
	}
}

func assertExternalOIDCTransitions(t *testing.T, fixture OIDCFixture) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, nonce, tokenMode := "", "", "exchange"
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			writer.Header().Set("Content-Type", "application/json")
			if tokenMode == "exchange" {
				http.Error(writer, "down", http.StatusServiceUnavailable)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"access","token_type":"Bearer","id_token":"invalid"}`))
			return
		}
		FakeOIDC(t, writer, request, issuer, nonce, private)
	}))
	t.Cleanup(provider.Close)
	issuer = provider.URL
	handler := fixture.NewHandler(t.TempDir(), federation.OIDCConfig{Issuer: issuer, ClientID: "kinosail", ClientSecret: "secret", RedirectURL: "http://localhost/login/oidc/callback"}, scim.Config{})
	for _, expected := range []int{http.StatusBadGateway, http.StatusUnauthorized} {
		start := httptest.NewRecorder()
		handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
		location := MustOIDCAuthorization(t, start)
		nonce = location.Query().Get("nonce")
		callback := httptest.NewRecorder()
		path := "/login/oidc/callback?code=bad&state=" + url.QueryEscape(location.Query().Get("state"))
		handler.ServeHTTP(callback, OIDCCallbackRequest(t, path, OIDCStateCookie(t, start)))
		if callback.Code != expected {
			t.Fatalf("mode %s = %d %q", tokenMode, callback.Code, callback.Body.String())
		}
		tokenMode = "invalid-token"
	}
	for method, expected := range map[string]int{http.MethodGet: http.StatusBadRequest, http.MethodPost: http.StatusUnauthorized} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, "/login/mfa?challenge=missing", strings.NewReader("challenge=missing&code=000000")))
		if response.Code != expected {
			t.Fatalf("%s MFA = %d", method, response.Code)
		}
	}
}

// ExternalBoundaryItemID retrieves an item from the real rendered library.
func ExternalBoundaryItemID(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(response.Body.String())
	if len(match) != 2 {
		t.Fatalf("library lacks item: %d %q", response.Code, response.Body.String())
	}
	return match[1]
}
