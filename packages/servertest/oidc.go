package servertest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/scim"
)

const (
	oidcSCIMToken      = "scim-test-token-012345678901234567890" //nolint:gosec // Test-only bearer token.
	oidcSCIMUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"
)

// OIDCFixture binds protocol contracts to each app's real server and account fixtures.
type OIDCFixture struct {
	NewHandler  func(string, federation.OIDCConfig, scim.Config) http.Handler
	StoredState func(*testing.T, string, string) []byte
	TOTP        func(*testing.T, string, time.Time) string
}

// OIDCContract runs the linking and provisioned-identity scenarios against a real app.
func OIDCContract(t *testing.T, fixture OIDCFixture) {
	t.Helper()
	t.Run("ValidatesAuthorizationCodeAndLinksProfile", func(t *testing.T) { AssertOIDCLinksProfile(t, fixture) })
	t.Run("LinksSCIMProfileByConfiguredIdentityClaim", func(t *testing.T) { AssertOIDCSCIMIdentity(t, fixture) })
}

// AssertOIDCLinksProfile checks explicit linking, session creation, MFA, and replay rejection.
func AssertOIDCLinksProfile(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, nonce := "", ""
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		FakeOIDC(t, writer, request, issuer, nonce, private)
	}))
	t.Cleanup(provider.Close)
	issuer = provider.URL
	dataDir := t.TempDir()
	handler := fixture.NewHandler(dataDir, federation.OIDCConfig{Issuer: issuer, ClientID: "kinosail", ClientSecret: "provider-secret", RedirectURL: "http://localhost/login/oidc/callback"}, scim.Config{})
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var result struct {
		Token string
		TOTP  struct{ Secret string }
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &result); err != nil {
		t.Fatalf("JSON = %d %q: %v", setup.Code, setup.Body.String(), err)
	}
	enabled := APICall(t, handler, result.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, result.TOTP.Secret, time.Now())})
	if enabled.Code != http.StatusOK || !strings.Contains(enabled.Body.String(), `"enabled":true`) {
		t.Fatalf("MFA setup = %d %q", enabled.Code, enabled.Body.String())
	}
	// Linking requires a recently authenticated session, not the enrollment session.
	confirmedLogin := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": fixture.TOTP(t, result.TOTP.Secret, time.Now())})
	AssertAPIBody(t, confirmedLogin, http.StatusCreated, `"mfaEnrollmentRequired":false`)
	var authenticated struct{ Token string }
	MustJSON(t, confirmedLogin, &authenticated)
	result.Token = authenticated.Token
	owner := &http.Cookie{Name: "__Host-kinosail_session", Value: result.Token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))
	mustOIDCLogin(t, login)

	assertUnlinkedOIDCIdentity(t, handler, &nonce)
	profiles := assertOIDCProfileLink(t, fixture, handler, dataDir, issuer, &nonce, owner)
	assertLinkedOIDCSessions(t, fixture, handler, &nonce, profiles, result.TOTP.Secret, owner)
}

// AssertOIDCSCIMIdentity checks profile linking through the configured identity claim.
func AssertOIDCSCIMIdentity(t *testing.T, fixture OIDCFixture) {
	t.Parallel()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, nonce := "", ""
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		FakeOIDC(t, writer, request, issuer, nonce, private)
	}))
	t.Cleanup(provider.Close)
	issuer = provider.URL
	dataDir := t.TempDir()
	handler := fixture.NewHandler(dataDir, federation.OIDCConfig{Issuer: issuer, ClientID: "kinosail", ClientSecret: "provider-secret", RedirectURL: "http://localhost/login/oidc/callback", IdentityClaim: "oid"}, scim.Config{Token: oidcSCIMToken, TokenExpiresAt: time.Now().Add(time.Hour)})
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password"})
	if setup.Code != http.StatusCreated {
		t.Fatalf("SCIM/OIDC setup = %d %q", setup.Code, setup.Body.String())
	}
	created := SCIMCall(t, handler, oidcSCIMToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{oidcSCIMUserSchema}, "userName": "jane@example.com", "externalId": "directory-1", "displayName": "Jane Viewer", "active": true,
		"emails": []map[string]any{{"value": "jane@example.com", "primary": true}},
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("SCIM/OIDC profile = %d %q", created.Code, created.Body.String())
	}

	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/oidc", nil))
	location := MustOIDCAuthorization(t, start)
	if location.Query().Get("scope") != "openid profile" {
		t.Fatalf("custom identity scope = %q", location.Query().Get("scope"))
	}
	nonce = location.Query().Get("nonce")
	callback := httptest.NewRecorder()
	handler.ServeHTTP(callback, OIDCCallbackRequest(t, "/login/oidc/callback?code=approved&state="+url.QueryEscape(location.Query().Get("state")), OIDCStateCookie(t, start)))
	assertOIDCProvisionedSession(t, fixture, callback, dataDir)
}

func assertOIDCProvisionedSession(t *testing.T, fixture OIDCFixture, callback *httptest.ResponseRecorder, dataDir string) {
	t.Helper()
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/" {
		t.Fatalf("claim-matched SCIM/OIDC callback = %d location=%q body=%q", callback.Code, callback.Header().Get("Location"), callback.Body.String())
	}
	var linked bool
	for _, cookie := range callback.Result().Cookies() {
		linked = linked || cookie.Name == "__Host-kinosail_session" && cookie.Value != ""
	}
	if !linked || !strings.Contains(string(fixture.StoredState(t, dataDir, "profiles.json")), `"oidcSubject":"directory-1"`) {
		t.Fatalf("SCIM/OIDC session cookies=%v profiles=%q", callback.Result().Cookies(), fixture.StoredState(t, dataDir, "profiles.json"))
	}
}
