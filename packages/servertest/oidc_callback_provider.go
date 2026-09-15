package servertest

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/scim"
)

type oidcCallbackProvider struct {
	issuer                                 string
	nonce                                  atomic.Value
	private                                *rsa.PrivateKey
	exchanges, basicAttempts, postAttempts atomic.Int32
	postOnly                               bool
}

func newOIDCCallbackProvider(t *testing.T, signed bool) *oidcCallbackProvider {
	t.Helper()
	provider := &oidcCallbackProvider{}
	provider.nonce.Store("")
	if signed {
		private, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		provider.private = private
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			provider.exchanges.Add(1)
			if provider.postOnly && !provider.acceptTokenPost(writer, request) {
				return
			}
		}
		FakeOIDC(t, writer, request, provider.issuer, provider.nonce.Load().(string), provider.private)
	}))
	t.Cleanup(server.Close)
	provider.issuer = server.URL
	return provider
}

func (provider *oidcCallbackProvider) acceptTokenPost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Header.Get("Authorization") != "" {
		provider.basicAttempts.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
		return false
	}
	provider.postAttempts.Add(1)
	if request.FormValue("client_id") != "kinosail" || request.FormValue("client_secret") != "provider-secret" {
		writer.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

func (provider *oidcCallbackProvider) handler(t *testing.T, fixture OIDCFixture, identityClaim string) (http.Handler, string) {
	t.Helper()
	dataDir := t.TempDir()
	handler := fixture.NewHandler(dataDir, federation.OIDCConfig{Issuer: provider.issuer, ClientID: "kinosail", ClientSecret: "provider-secret", RedirectURL: "http://localhost/login/oidc/callback", IdentityClaim: identityClaim}, scim.Config{})
	return handler, dataDir
}

func (provider *oidcCallbackProvider) start(t *testing.T, handler http.Handler) (*url.URL, *http.Cookie) {
	t.Helper()
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
	authorization := MustOIDCAuthorization(t, start)
	provider.nonce.Store(authorization.Query().Get("nonce"))
	return authorization, OIDCStateCookie(t, start)
}

func callbackOIDC(t *testing.T, handler http.Handler, query string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, OIDCCallbackRequest(t, "/login/oidc/callback?"+query, cookie))
	return response
}
