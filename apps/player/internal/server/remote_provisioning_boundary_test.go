package server_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/scim"
)

func TestPublicListenerBlocksProvisioningEvenWithValidCredentials(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, SCIM: server.SCIMConfig{Token: scimTestToken, TokenExpiresAt: time.Now().Add(time.Hour)}})
	_ = signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	input := map[string]any{"schemas": []string{scimUserSchema}, "userName": "viewer@example.com", "active": true}
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", input)
	var viewer struct{ ID string }
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &viewer) != nil || viewer.ID == "" {
		t.Fatalf("local provisioning = %d %q", created.Code, created.Body.String())
	}
	before := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("local directory = %d", before.Code)
	}
	public := server.Remote(handler)
	for _, credential := range []string{"", "invalid-token", scimTestToken} {
		for _, pattern := range scim.Patterns() {
			method, path, _ := strings.Cut(pattern, " ")
			path = strings.ReplaceAll(path, "{id}", viewer.ID)
			if method == http.MethodGet {
				assertRemoteProvisioningDenied(t, public, method, path, "", credential)
				assertRemoteProvisioningDenied(t, public, http.MethodHead, path, "", credential)
			} else {
				assertRemoteProvisioningDenied(t, public, method, path, `{"schemas":["`+scimUserSchema+`"],"userName":"changed@example.com","active":false}`, credential)
			}
		}
	}
	after := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil)
	if after.Code != http.StatusOK || after.Body.String() != before.Body.String() {
		t.Fatalf("public request changed the local directory: before=%s after=%s", before.Body.String(), after.Body.String())
	}
}

func assertRemoteProvisioningDenied(t *testing.T, handler http.Handler, method, path, body, token string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.TLS = &tls.ConnectionState{}
	request.Header.Set("Content-Type", "application/scim+json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	// Neither forwarding headers nor a claimed loopback source can turn public ingress local.
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	request.Header.Set("X-Kinosail-Remote", "false")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("public %s %s = %d %q", method, path, response.Code, response.Body.String())
	}
}
