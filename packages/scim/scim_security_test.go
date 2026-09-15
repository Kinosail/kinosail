package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSCIMRejectsInsecureAndAmbiguousRequestsWithoutMutation(t *testing.T) { //nolint:cyclop,funlen // Boundary failures are proven against the public protocol seam.
	t.Parallel()
	handler := serverWithSCIM(t)
	insecure := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scim/v2/Users", nil)
	insecure.Header.Set("Authorization", "Bearer "+scimTestToken)
	insecureResponse := httptest.NewRecorder()
	handler.ServeHTTP(insecureResponse, insecure)
	if insecureResponse.Code != http.StatusBadRequest {
		t.Fatalf("insecure SCIM request = %d %q", insecureResponse.Code, insecureResponse.Body.String())
	}
	conflictingPrimary := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "ambiguous@example.com", "emails": []map[string]any{{"value": "one@example.com", "primary": true}, {"value": "two@example.com", "primary": true}},
	})
	if conflictingPrimary.Code != http.StatusBadRequest {
		t.Fatalf("conflicting primary emails = %d %q", conflictingPrimary.Code, conflictingPrimary.Body.String())
	}
	first := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "first@example.com", "externalId": "provider-scoped-id"})
	second := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "second@example.com", "externalId": "provider-scoped-id"})
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("provisioner-scoped externalId first=%d second=%d", first.Code, second.Code)
	}
	var resource struct{ ID string }
	if err := json.Unmarshal(first.Body.Bytes(), &resource); err != nil {
		t.Fatal(err)
	}
	for name, headers := range map[string]map[string]string{
		"conflicting": {"If-Match": first.Header().Get("ETag"), "If-None-Match": `W/"stale"`},
		"malformed":   {"If-Match": "not-an-entity-tag"},
		"oversized":   {"If-Match": strings.Repeat("x", 1025)},
	} {
		response := scimCallWithHeaders(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resource.ID, map[string]any{"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "displayName", "value": "Changed"}}}, headers)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s conditional request = %d %q", name, response.Code, response.Body.String())
		}
	}
	list := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"totalResults":2`) || strings.Contains(list.Body.String(), `"displayName":"Changed"`) || strings.Contains(list.Body.String(), "ambiguous@example.com") {
		t.Fatalf("SCIM state after rejected requests = %d %q", list.Code, list.Body.String())
	}
}

func TestSCIMDiscoveryRejectsUnknownQueriesAndResources(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	for path, status := range map[string]int{
		"/scim/v2/ServiceProviderConfig?unknown=1": http.StatusBadRequest,
		"/scim/v2/ResourceTypes?unknown=1":         http.StatusBadRequest,
		"/scim/v2/ResourceTypes/User?unknown=1":    http.StatusBadRequest,
		"/scim/v2/Schemas?unknown=1":               http.StatusBadRequest,
		"/scim/v2/Schemas/unknown":                 http.StatusNotFound,
		"/scim/v2/ResourceTypes/unknown":           http.StatusNotFound,
	} {
		response := scimCall(t, handler, scimTestToken, http.MethodGet, path, nil)
		if response.Code != status {
			t.Fatalf("GET %s = %d %q, want %d", path, response.Code, response.Body.String(), status)
		}
	}
	for _, path := range []string{
		"/scim/v2/ServiceProviderConfig?startIndex=1&count=100",
		"/scim/v2/ResourceTypes?filter=id%20eq%20%22User%22&sortBy=name&sortOrder=ascending",
		"/scim/v2/Schemas?attributes=id,name",
	} {
		if response := scimCall(t, handler, scimTestToken, http.MethodGet, path, nil); response.Code != http.StatusOK {
			t.Fatalf("provider discovery query %s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

func TestSCIMMissingMutationResourcesDoNotChangeRepository(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	for _, request := range []struct {
		method string
		body   any
	}{
		{http.MethodPut, map[string]any{"schemas": []string{scimUserSchema}, "userName": "missing@example.com"}},
		{http.MethodPatch, map[string]any{"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "active", "value": false}}}},
		{http.MethodDelete, nil},
	} {
		response := scimCall(t, handler, scimTestToken, request.method, "/scim/v2/Users/missing", request.body)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s missing resource = %d %q", request.method, response.Code, response.Body.String())
		}
	}
	list := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"totalResults":0`) {
		t.Fatalf("repository changed after missing mutations = %d %q", list.Code, list.Body.String())
	}
}
