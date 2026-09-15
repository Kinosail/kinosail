package scim

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const scimTestToken = "scim-test-token-012345678901234567890" //nolint:gosec // Test-only bearer token.

func TestSCIMProvisioningLifecycleAndBoundaries(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	assertSCIMDiscovery(t, handler)
	resourceID, entityTag := createSCIMLifecycleProfile(t, handler)
	assertSCIMLifecycleQueries(t, handler, resourceID)
	assertSCIMLifecycleConditionals(t, handler, resourceID, entityTag)
	assertSCIMLifecyclePatchBoundaries(t, handler, resourceID)
	assertSCIMLifecycleMutations(t, handler, resourceID)
}

func TestSCIMRejectsExpiredBearerToken(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIMConfig(Config{Token: scimTestToken, TokenExpiresAt: time.Now().Add(-time.Second)})
	if response := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/ServiceProviderConfig", nil); response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expired SCIM token = %d authenticate=%q body=%q", response.Code, response.Header().Get("WWW-Authenticate"), response.Body.String())
	}
}

func TestSCIMIfMatchSerializesConcurrentMutations(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "concurrent@example.com", "displayName": "Concurrent"})
	if created.Code != http.StatusCreated {
		t.Fatalf("concurrent test create = %d %q", created.Code, created.Body.String())
	}
	var resource struct{ ID string }
	if err := json.Unmarshal(created.Body.Bytes(), &resource); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	responses := make(chan int, 2)
	for _, name := range []string{"First", "Second"} {
		go func(name string) {
			<-start
			body := map[string]any{"schemas": []string{scimUserSchema}, "userName": "concurrent@example.com", "displayName": name, "active": true}
			response := scimCallWithHeaders(t, handler, scimTestToken, http.MethodPut, "/scim/v2/Users/"+resource.ID, body, map[string]string{"If-Match": created.Header().Get("ETag")})
			responses <- response.Code
		}(name)
	}
	close(start)
	statuses := []int{<-responses, <-responses}
	if (statuses[0] != http.StatusOK && statuses[0] != http.StatusPreconditionFailed) || (statuses[1] != http.StatusOK && statuses[1] != http.StatusPreconditionFailed) || statuses[0] == statuses[1] {
		t.Fatalf("concurrent If-Match statuses = %#v", statuses)
	}
}

func TestSCIMRejectsDuplicateAuthorizationHeadersAndAuthenticatesPublicAccess(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users", nil)
	request.Header.Add("Authorization", "Bearer "+scimTestToken)
	request.Header.Add("Authorization", "Bearer another-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate SCIM authorization = %d %q", response.Code, response.Body.String())
	}
	remote := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	remote.TLS = &tls.ConnectionState{}
	remote.Header.Set("Authorization", "Bearer "+scimTestToken)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, remote)
	if response.Code != http.StatusOK {
		t.Fatalf("remote SCIM access = %d %q", response.Code, response.Body.String())
	}
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "remote@example.com"})
	if created.Code != http.StatusCreated {
		t.Fatalf("remote SCIM mutation = %d %q", created.Code, created.Body.String())
	}
	if list := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil); list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"totalResults":1`) {
		t.Fatalf("SCIM state after remote mutation = %d %q", list.Code, list.Body.String())
	}
}

func TestSCIMRejectsInvalidFiltersAndPagesWithoutMutation(t *testing.T) { //nolint:cyclop // Boundary cases intentionally remain explicit and side-effect observable.
	t.Parallel()
	handler := serverWithSCIM(t)
	for _, path := range []string{"/scim/v2/Users?filter=name%20eq%20%22bad%22", "/scim/v2/Users?filter=urn%3Aietf%3Aparams%3Ascim%3Aschemas%3Acore%3A2.0%3AUser%3Aunknown%20eq%20%22bad%22", "/scim/v2/Users?count=201", "/scim/v2/Users?count=1&count=2", "/scim/v2/Users?sort=id", "/scim/v2/Users?attributes=unknown", "/scim/v2/Users?attributes=userName&excludedAttributes=userName", "/scim/v2/Users?attributes=userName&excludedAttributes=displayName", "/scim/v2/Users?startIndex=0", "/scim/v2/Users?count=-1"} {
		response := scimCall(t, handler, scimTestToken, http.MethodGet, path, nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid SCIM query %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	for name, body := range map[string]json.RawMessage{
		"malformed":          json.RawMessage(`{"schemas":["` + scimUserSchema + `"],"userName":`),
		"duplicate-field":    json.RawMessage(`{"schemas":["` + scimUserSchema + `"],"userName":"first","USERNAME":"second","displayName":"Invalid"}`),
		"null-username":      json.RawMessage(`{"schemas":["` + scimUserSchema + `"],"userName":null}`),
		"duplicate-schema":   json.RawMessage(`{"schemas":["` + scimUserSchema + `","` + scimUserSchema + `"],"userName":"invalid@example.com"}`),
		"unsupported-schema": json.RawMessage(`{"schemas":["urn:example:unsupported"],"userName":"invalid@example.com"}`),
	} {
		response := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid SCIM %s = %d %q", name, response.Code, response.Body.String())
		}
	}
	if response := scimCallWithContentType(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", "text/plain", map[string]any{"schemas": []string{scimUserSchema}, "userName": "wrong-content-type@example.com"}); response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported SCIM content type = %d %q", response.Code, response.Body.String())
	}
	if response := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", json.RawMessage(`{"schemas":["`+scimUserSchema+`"],"userName":"`+strings.Repeat("x", 1<<20)+`"}`)); response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized SCIM body = %d %q", response.Code, response.Body.String())
	}
	if response := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "", "displayName": "Invalid"}); response.Code != http.StatusBadRequest {
		t.Fatalf("missing SCIM username = %d %q", response.Code, response.Body.String())
	}
	if response := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"totalResults":0`) {
		t.Fatalf("SCIM list after invalid operations = %d %q", response.Code, response.Body.String())
	}
}
