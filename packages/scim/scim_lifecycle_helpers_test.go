package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertSCIMDiscovery(t *testing.T, handler http.Handler) {
	t.Helper()
	unauthorized := scimCall(t, handler, "wrong", http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	assertSCIMResponse(t, "invalid authentication", unauthorized, http.StatusUnauthorized, nil, nil)
	if unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("SCIM invalid authentication omitted challenge")
	}
	assertSCIMResponse(t, "service provider config", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/ServiceProviderConfig", nil), http.StatusOK, []string{`"patch":{"supported":true}`}, nil)
	assertSCIMResponse(t, "resource types", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/ResourceTypes", nil), http.StatusOK, []string{`"Resources":[`, `"endpoint":"/scim/v2/Users"`}, nil)
	assertSCIMResponse(t, "schemas", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Schemas", nil), http.StatusOK, []string{`"Resources":[`, scimUserSchema}, nil)
	assertSCIMResponse(t, "resource type", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/ResourceTypes/User", nil), http.StatusOK, []string{`"endpoint":"/scim/v2/Users"`}, nil)
	assertSCIMResponse(t, "schema", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Schemas/"+scimUserSchema, nil), http.StatusOK, []string{`"id":"` + scimUserSchema + `"`, `"uniqueness":"none"`}, nil)
}

func createSCIMLifecycleProfile(t *testing.T, handler http.Handler) (string, string) { //nolint:cyclop // The response fixture validates one complete resource representation.
	t.Helper()
	invalid := map[string]any{"schemas": []string{scimUserSchema}, "displayName": "No username", "unknown": true}
	assertSCIMResponse(t, "invalid create", scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", invalid), http.StatusBadRequest, nil, nil)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "jane@example.com", "externalId": "directory-1", "displayName": "Jane Viewer", "active": true, "emails": []map[string]any{{"value": "jane@example.com", "primary": true}},
		"name": map[string]string{"givenName": "Jane", "familyName": "Viewer"},
	})
	if created.Code != http.StatusCreated || created.Header().Get("Location") == "" || created.Header().Get("Content-Location") != created.Header().Get("Location") || created.Header().Get("ETag") == "" || created.Header().Get("Content-Type") != "application/scim+json" {
		t.Fatalf("SCIM create = %d location=%q content-location=%q etag=%q content-type=%q body=%q", created.Code, created.Header().Get("Location"), created.Header().Get("Content-Location"), created.Header().Get("ETag"), created.Header().Get("Content-Type"), created.Body.String())
	}
	var resource struct {
		ID, UserName, DisplayName, ExternalID string
		Active                                bool
		Meta                                  struct{ Location, Version string }
		Emails                                []struct{ Value string }
	}
	err := json.Unmarshal(created.Body.Bytes(), &resource)
	if err != nil || resource.ID == "" || resource.UserName != "jane@example.com" || resource.DisplayName != "Jane Viewer" || resource.ExternalID != "directory-1" || len(resource.Emails) != 1 || resource.Emails[0].Value != "jane@example.com" || !resource.Active || resource.Meta.Location != created.Header().Get("Content-Location") || resource.Meta.Version != created.Header().Get("ETag") {
		t.Fatalf("SCIM resource = %+v err=%v body=%q", resource, err, created.Body.String())
	}
	return resource.ID, created.Header().Get("ETag")
}

func assertSCIMLifecycleQueries(t *testing.T, handler http.Handler, resourceID string) {
	t.Helper()
	duplicate := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "JANE@example.com", "displayName": "Another Jane"})
	assertSCIMResponse(t, "duplicate user", duplicate, http.StatusConflict, []string{`"scimType":"uniqueness"`}, nil)
	assertSCIMResponse(t, "filtered list", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users?filter=userName%20eq%20%22jane%40example.com%22&startIndex=1&count=1", nil), http.StatusOK, []string{`"totalResults":1`, `"itemsPerPage":1`}, nil)
	assertSCIMResponse(t, "external ID case", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users?filter=externalId%20eq%20%22DIRECTORY-1%22", nil), http.StatusOK, []string{`"totalResults":0`}, nil)
	for _, filter := range []string{`displayName%20eq%20%22Jane%20Viewer%22`, `userName%20sw%20%22jane%22`, `displayName%20co%20%22Viewer%22`} {
		assertSCIMResponse(t, "supported filter", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users?filter="+filter, nil), http.StatusOK, []string{`"totalResults":1`}, nil)
	}
	selected := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID+"?attributes=userName", nil)
	assertSCIMResponse(t, "attribute selection", selected, http.StatusOK, []string{`"userName":"jane@example.com"`}, []string{`"displayName"`, `"meta"`})
	subselected := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID+"?attributes=name.givenName,emails.value", nil)
	assertSCIMResponse(t, "subattribute selection", subselected, http.StatusOK, []string{`"name":{"givenName":"Jane"}`, `"emails":[{"value":"jane@example.com"}]`}, []string{`"displayName"`, `"familyName"`})
}

func assertSCIMLifecycleConditionals(t *testing.T, handler http.Handler, resourceID, entityTag string) {
	t.Helper()
	conditional := scimCallWithHeaders(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil, map[string]string{"If-None-Match": `W/"stale", ` + entityTag})
	if conditional.Code != http.StatusNotModified || conditional.Header().Get("ETag") != entityTag {
		t.Fatalf("SCIM If-None-Match = %d etag=%q body=%q", conditional.Code, conditional.Header().Get("ETag"), conditional.Body.String())
	}
	stale := scimCallWithHeaders(t, handler, scimTestToken, http.MethodPut, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "jane@example.com", "displayName": "Should Not Apply", "active": false,
	}, map[string]string{"If-Match": `W/"0"`})
	assertSCIMResponse(t, "stale If-Match", stale, http.StatusPreconditionFailed, nil, nil)
	blocked := scimCallWithHeaders(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "displayName", "value": "Should Not Apply"}},
	}, map[string]string{"If-None-Match": entityTag})
	assertSCIMResponse(t, "matching If-None-Match", blocked, http.StatusPreconditionFailed, nil, nil)
	unchanged := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil)
	assertSCIMResponse(t, "stale mutation state", unchanged, http.StatusOK, []string{`"active":true`}, []string{"Should Not Apply"})
	assertSCIMResponse(t, "invalid resource ID", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/bad%00", nil), http.StatusBadRequest, nil, nil)
}

func assertSCIMLifecyclePatchBoundaries(t *testing.T, handler http.Handler, resourceID string) {
	t.Helper()
	unknown := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "value": map[string]any{"unknown": "value"}}},
	})
	assertSCIMResponse(t, "unknown patch", unknown, http.StatusBadRequest, nil, nil)
	withoutPath := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "remove", "value": map[string]any{"displayName": true}}},
	})
	assertSCIMResponse(t, "remove without path", withoutPath, http.StatusBadRequest, []string{`"scimType":"noTarget"`}, nil)
	unchanged := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil)
	assertSCIMResponse(t, "remove without path state", unchanged, http.StatusOK, []string{`"displayName":"Jane Viewer"`}, nil)
	nullPath := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": nil, "value": true}},
	})
	assertSCIMResponse(t, "null patch path", nullPath, http.StatusBadRequest, nil, nil)
	unchanged = scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil)
	assertSCIMResponse(t, "null patch path state", unchanged, http.StatusOK, []string{`"active":true`}, nil)
	nullValue := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "active", "value": nil}},
	})
	assertSCIMResponse(t, "null patch value", nullValue, http.StatusOK, []string{`"active":true`}, nil)
	invalidNamespace := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "evil:active", "value": false}},
	})
	assertSCIMResponse(t, "invalid namespace", invalidNamespace, http.StatusBadRequest, nil, nil)
	unchanged = scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil)
	assertSCIMResponse(t, "invalid namespace state", unchanged, http.StatusOK, []string{`"active":true`, `"displayName":"Jane Viewer"`}, nil)
}

func assertSCIMLifecycleMutations(t *testing.T, handler http.Handler, resourceID string) {
	t.Helper()
	valuePath := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "add", "path": `emails[type eq "work"].value`, "value": "work@example.com"}, {"op": "replace", "path": `emails[type eq "work"].primary`, "value": true}},
	})
	assertSCIMResponse(t, "email valuePath", valuePath, http.StatusOK, []string{`"value":"work@example.com"`, `"primary":true`}, nil)
	namespaced := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": scimUserSchema + ":active", "value": true}},
	})
	assertSCIMResponse(t, "namespaced patch", namespaced, http.StatusOK, []string{`"active":true`}, nil)
	patched := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "Replace", "path": "active", "value": false}, {"op": "replace", "path": "displayName", "value": "Jane Disabled"}},
	})
	assertSCIMResponse(t, "patch", patched, http.StatusOK, []string{`"displayName":"Jane Disabled"`}, []string{`"active":true`})
	removed := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "remove", "path": "name"}},
	})
	var resource struct {
		DisplayName string
		Name        struct{ Formatted, GivenName, FamilyName string }
	}
	if err := json.Unmarshal(removed.Body.Bytes(), &resource); err != nil || removed.Code != http.StatusOK || resource.Name != (struct{ Formatted, GivenName, FamilyName string }{}) || resource.DisplayName != "Jane Disabled" {
		t.Fatalf("SCIM name removal = %d resource=%#v body=%q err=%v", removed.Code, resource, removed.Body.String(), err)
	}
	replaced := scimCall(t, handler, scimTestToken, http.MethodPut, "/scim/v2/Users/"+resourceID, map[string]any{
		"schemas": []string{scimUserSchema}, "id": "client-assigned-id", "userName": "jane@example.com", "displayName": "Jane Reenabled", "externalId": "directory-2", "active": true,
	})
	assertSCIMResponse(t, "replace", replaced, http.StatusOK, []string{`"id":"` + resourceID + `"`, `"active":true`, `"externalId":"directory-2"`}, nil)
	deleted := scimCall(t, handler, scimTestToken, http.MethodDelete, "/scim/v2/Users/"+resourceID, nil)
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("SCIM delete = %d %q", deleted.Code, deleted.Body.String())
	}
	assertSCIMResponse(t, "deleted user", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resourceID, nil), http.StatusNotFound, nil, nil)
	assertSCIMResponse(t, "deleted list", scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users", nil), http.StatusOK, nil, []string{resourceID})
	restored := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "jane@example.com", "displayName": "Jane Restored", "active": true})
	assertSCIMResponse(t, "reprovision", restored, http.StatusCreated, []string{`"id":"` + resourceID + `"`}, nil)
}

func assertSCIMResponse(t *testing.T, name string, response *httptest.ResponseRecorder, status int, contains, excludes []string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("SCIM %s = %d %q, want %d", name, response.Code, response.Body.String(), status)
	}
	for _, value := range contains {
		if value != "" && !strings.Contains(response.Body.String(), value) {
			t.Fatalf("SCIM %s missing %q: %q", name, value, response.Body.String())
		}
	}
	for _, value := range excludes {
		if strings.Contains(response.Body.String(), value) {
			t.Fatalf("SCIM %s unexpectedly contains %q: %q", name, value, response.Body.String())
		}
	}
}
