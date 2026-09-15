package servertest

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// HomeAssistantIsDarkByDefault verifies the disabled API and both visible setup surfaces.
func (fixture LibraryAPIFixture) HomeAssistantIsDarkByDefault(t *testing.T) {
	t.Parallel()
	handler, owner := fixture.Server(t)

	AssertAPIBody(t, APICall(t, handler, owner, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"homeAssistant":false`)
	AssertAPIBody(t, APICall(t, handler, "", http.MethodGet, "/api/v1/home-assistant", nil), http.StatusNotFound, `"error":"not found"`)
	AssertAPIBody(t, APICall(t, handler, owner, http.MethodGet, "/home-assistant/authorize", nil), http.StatusNotFound)
	AssertAPIBody(t, homeAssistantFormCall(t, handler, "", "/api/v1/home-assistant/token", url.Values{}), http.StatusNotFound)

	for name, response := range map[string]*httptest.ResponseRecorder{
		"settings": APICall(t, handler, owner, http.MethodGet, "/settings", nil),
		"wizard":   APICall(t, handler, owner, http.MethodGet, "/onboarding/connection", nil),
	} {
		body := response.Body.String()
		if response.Code != http.StatusOK || !strings.Contains(body, "Home Assistant") || strings.Contains(body, `name="enabled" value="true" checked data-home-assistant`) {
			t.Fatalf("%s default Home Assistant control = %d %q", name, response.Code, body)
		}
	}
}

// HomeAssistantBrowserApproval verifies PKCE, replay protection, and scoped access.
func (fixture LibraryAPIFixture) HomeAssistantBrowserApproval(t *testing.T) { //nolint:cyclop // One flow proves approval, PKCE, replay, and scope.
	t.Parallel()
	handler, owner := fixture.Server(t)
	AssertAPIBody(t, APICall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK)
	verifier := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type": {"code"}, "client_id": {"home-assistant"},
		"redirect_uri": {"http://homeassistant.local:8123/auth/external/callback"}, "state": {"signed-state"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"},
	}
	signIn := APICall(t, handler, "", http.MethodGet, "/home-assistant/authorize?"+query.Encode(), nil)
	if signIn.Code != http.StatusSeeOther || !strings.HasPrefix(signIn.Header().Get("Location"), "/login?next=") {
		t.Fatalf("approval sign-in = %d %q", signIn.Code, signIn.Header().Get("Location"))
	}
	approval := APICall(t, handler, owner, http.MethodGet, "/home-assistant/authorize?"+query.Encode(), nil)
	if approval.Code != http.StatusOK || !strings.Contains(approval.Body.String(), "Allow Home Assistant to connect?") || !strings.Contains(approval.Body.String(), "homeassistant.local:8123") {
		t.Fatalf("approval = %d %q", approval.Code, approval.Body.String())
	}
	match := regexp.MustCompile(`name="request" value="([A-Za-z0-9_-]+)"`).FindStringSubmatch(approval.Body.String())
	if len(match) != 2 {
		t.Fatalf("approval has no bounded transaction: %q", approval.Body.String())
	}
	approved := homeAssistantFormCall(t, handler, owner, "/home-assistant/authorize", url.Values{"request": {match[1]}, "decision": {"allow"}})
	redirect, err := approved.Result().Location()
	if approved.Code != http.StatusFound || err != nil || redirect.Query().Get("state") != "signed-state" || redirect.Query().Get("code") == "" {
		t.Fatalf("approval redirect = %d %v %v", approved.Code, redirect, err)
	}
	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {"home-assistant"}, "code": {redirect.Query().Get("code")},
		"redirect_uri": {"http://homeassistant.local:8123/auth/external/callback"}, "code_verifier": {strings.Repeat("x", 64)},
	}
	AssertAPIBody(t, homeAssistantFormCall(t, handler, "", "/api/v1/home-assistant/token", form), http.StatusBadRequest, `"error":"invalid_grant"`)
	form.Set("code_verifier", verifier)
	exchanged := homeAssistantFormCall(t, handler, "", "/api/v1/home-assistant/token", form)
	var credential struct {
		AccessToken string `json:"access_token"`
	}
	MustJSON(t, exchanged, &credential)
	if exchanged.Code != http.StatusOK || !strings.HasPrefix(credential.AccessToken, "ks_") {
		t.Fatalf("token exchange = %d %#v", exchanged.Code, credential)
	}
	AssertAPIBody(t, homeAssistantFormCall(t, handler, "", "/api/v1/home-assistant/token", form), http.StatusBadRequest, `"error":"invalid_grant"`)
	AssertAPIBody(t, APICall(t, handler, credential.AccessToken, http.MethodGet, "/api/v1/home-assistant/library", nil), http.StatusOK, `"items"`)
	AssertAPIBody(t, APICall(t, handler, credential.AccessToken, http.MethodGet, "/api/v1/settings", nil), http.StatusForbidden)
}

func homeAssistantFormCall(t *testing.T, handler http.Handler, token, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// HomeAssistantRouteEnvironment returns the bounded test configuration values.
func HomeAssistantRouteEnvironment(key string) (string, bool) {
	values := map[string]string{
		"KINOSAIL_HOME_ASSISTANT_ENABLED": "true",
		"KINOSAIL_DUCKDNS_HTTPS":          testDuckDNSHTTPS,
	}
	value, ok := values[key]
	return value, ok
}

// JellyfinRouteEnvironment returns only the HTTPS value used by its route fixture.
func JellyfinRouteEnvironment(key string) (string, bool) {
	if key == "KINOSAIL_DUCKDNS_HTTPS" {
		return testDuckDNSHTTPS, true
	}
	return "", false
}

var testDuckDNSHTTPS = `{"domain":"family-media","token":"` + strings.Repeat("t", 32) + `","address":"192.168.1.10","termsAccepted":true}`
