package scim

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSCIMAcceptsAndNegotiatesProviderJSONMediaType(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCallWithContentType(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", "application/json", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "okta@example.com", "displayName": "Okta Viewer", "active": true,
		"name": map[string]string{"givenName": "Okta", "familyName": "Viewer"}, "emails": []map[string]any{{"primary": true, "value": "okta@example.com", "type": "work"}},
	})
	if created.Code != http.StatusCreated || created.Header().Get("Content-Type") != "application/json" || !strings.Contains(created.Body.String(), `"userName":"okta@example.com"`) {
		t.Fatalf("provider JSON SCIM create = %d content-type=%q %q", created.Code, created.Header().Get("Content-Type"), created.Body.String())
	}
}

func TestSCIMAcceptsAndDoesNotReturnOktaWriteOnlyPassword(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "okta-password@example.com", "password": "provider-generated-secret", "active": true,
	})
	if created.Code != http.StatusCreated || strings.Contains(created.Body.String(), "provider-generated-secret") || strings.Contains(created.Body.String(), `"password"`) {
		t.Fatalf("Okta password create = %d %q", created.Code, created.Body.String())
	}
}

func TestSCIMAcceptsStandardOptionalAndNullProviderAttributes(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "standard@example.com", "active": true,
		"name":  map[string]any{"formatted": "Standard Viewer", "givenName": "Standard", "familyName": "Viewer", "middleName": nil, "honorificPrefix": nil},
		"title": "Engineer", "userType": "Employee", "nickName": nil, "profileUrl": nil,
		"preferredLanguage": nil, "locale": nil, "timezone": nil, "phoneNumbers": nil, "addresses": nil, "photos": []any{}, "ims": []any{}, "entitlements": []any{}, "x509Certificates": []any{},
	})
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"userName":"standard@example.com"`) {
		t.Fatalf("standard provider create = %d %q", created.Code, created.Body.String())
	}
}

func TestSCIMRejectsMalformedIgnoredProviderAttributesWithoutCreatingProfile(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	rejected := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "malformed-optional@example.com", "phoneNumbers": map[string]string{"value": "not-an-array"},
	})
	lookup := scimCall(t, handler, scimTestToken, http.MethodGet, `/scim/v2/Users?filter=userName%20eq%20%22malformed-optional%40example.com%22`, nil)
	if rejected.Code != http.StatusBadRequest || !strings.Contains(lookup.Body.String(), `"totalResults":0`) {
		t.Fatalf("malformed optional attribute = %d lookup=%d %q", rejected.Code, lookup.Code, lookup.Body.String())
	}
}

func TestSCIMMatchesProviderEmailAndCompoundFilters(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "filter@example.com", "active": true,
		"emails": []map[string]any{{"value": "work@example.com", "type": "work", "primary": true}},
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("filter fixture create = %d %q", created.Code, created.Body.String())
	}
	for _, filter := range []string{
		`emails.value eq "work@example.com"`,
		`emails[type eq "work"].value eq "work@example.com"`,
		`userName eq "filter@example.com" and active eq true`,
	} {
		response := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users?filter="+url.QueryEscape(filter), nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"totalResults":1`) {
			t.Fatalf("provider filter %q = %d %q", filter, response.Code, response.Body.String())
		}
	}
}

func TestSCIMDiscoversEnterpriseUserExtension(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	for _, path := range []string{"/scim/v2/Schemas", "/scim/v2/Schemas/" + scimEnterpriseSchema, "/scim/v2/ResourceTypes/User"} {
		response := scimCall(t, handler, scimTestToken, http.MethodGet, path, nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), scimEnterpriseSchema) {
			t.Fatalf("enterprise discovery %s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

func TestSCIMPersistsEntraManagerPatch(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema, scimEnterpriseSchema}, "userName": "manager-patch@example.com",
	})
	var resource struct {
		ID string `json:"id"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &resource) != nil || resource.ID == "" {
		t.Fatalf("manager fixture create = %d %q", created.Code, created.Body.String())
	}
	patched := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resource.ID, map[string]any{
		"schemas":    []string{scimPatchSchema},
		"Operations": []map[string]any{{"op": "replace", "path": scimEnterpriseSchema + ":manager", "value": map[string]string{"value": "manager-id", "displayName": "Manager Viewer"}}},
	})
	read := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resource.ID, nil)
	if patched.Code != http.StatusOK || read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"manager":{"value":"manager-id","displayName":"Manager Viewer"}`) {
		t.Fatalf("Entra manager patch = %d read=%d %q", patched.Code, read.Code, read.Body.String())
	}
}

func TestSCIMProjectsEnterpriseAttributes(t *testing.T) { //nolint:cyclop // One interoperability scenario covers enterprise projection fields.
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema, scimEnterpriseSchema}, "userName": "projection@example.com",
		scimEnterpriseSchema: map[string]any{"department": "Media", "employeeNumber": "123", "manager": map[string]string{"value": "manager-id", "displayName": "Manager Viewer"}},
	})
	var resource struct{ ID string }
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &resource) != nil || resource.ID == "" {
		t.Fatalf("enterprise projection fixture = %d %q", created.Code, created.Body.String())
	}
	selected := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resource.ID+"?attributes="+url.QueryEscape(scimEnterpriseSchema+":department"), nil)
	manager := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resource.ID+"?attributes="+url.QueryEscape(scimEnterpriseSchema+":manager.displayName"), nil)
	excluded := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resource.ID+"?excludedAttributes="+url.QueryEscape(scimEnterpriseSchema), nil)
	partial := scimCall(t, handler, scimTestToken, http.MethodGet, "/scim/v2/Users/"+resource.ID+"?excludedAttributes="+url.QueryEscape(scimEnterpriseSchema+":department"), nil)
	if selected.Code != http.StatusOK || !strings.Contains(selected.Body.String(), `"department":"Media"`) || strings.Contains(selected.Body.String(), "employeeNumber") || strings.Contains(selected.Body.String(), "manager-id") {
		t.Fatalf("selected enterprise projection = %d %q", selected.Code, selected.Body.String())
	}
	if manager.Code != http.StatusOK || !strings.Contains(manager.Body.String(), `"manager":{"displayName":"Manager Viewer"}`) || strings.Contains(manager.Body.String(), "manager-id") {
		t.Fatalf("selected manager projection = %d %q", manager.Code, manager.Body.String())
	}
	if excluded.Code != http.StatusOK || strings.Contains(excluded.Body.String(), scimEnterpriseSchema+`":{`) {
		t.Fatalf("excluded enterprise projection = %d %q", excluded.Code, excluded.Body.String())
	}
	if partial.Code != http.StatusOK || strings.Contains(partial.Body.String(), `"department"`) || !strings.Contains(partial.Body.String(), `"employeeNumber":"123"`) {
		t.Fatalf("partial enterprise projection = %d %q", partial.Code, partial.Body.String())
	}
}

func TestSCIMClearsOptionalAttributesWithNullPatch(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "null-clear@example.com", "externalId": "directory-id",
	})
	var resource struct {
		ID string `json:"id"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &resource) != nil {
		t.Fatalf("null fixture create = %d %q", created.Code, created.Body.String())
	}
	patched := scimCall(t, handler, scimTestToken, http.MethodPatch, "/scim/v2/Users/"+resource.ID, map[string]any{
		"schemas": []string{scimPatchSchema}, "Operations": []map[string]any{{"op": "replace", "path": "externalId", "value": nil}},
	})
	if patched.Code != http.StatusOK || strings.Contains(patched.Body.String(), `"externalId"`) {
		t.Fatalf("null optional patch = %d %q", patched.Code, patched.Body.String())
	}
}

func TestSCIMIgnoresProviderRolesWithoutGrantingOwnerAccess(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema}, "userName": "directory-role@example.com", "roles": []map[string]string{{"value": "admin"}},
	})
	if created.Code != http.StatusCreated || strings.Contains(created.Body.String(), `"roles"`) {
		t.Fatalf("directory roles create = %d %q", created.Code, created.Body.String())
	}
}

func TestSCIMAcceptsEntraUserShapeWithoutIgnoringRoles(t *testing.T) {
	t.Parallel()
	handler := serverWithSCIM(t)
	const enterpriseSchema = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema, enterpriseSchema}, "externalId": "0a21f0f2-8d2a-4f8e-bf98-7363c4aed4ef", "userName": "entra@example.com", "active": true,
		"emails": []map[string]any{{"primary": true, "type": "work", "value": "entra@example.com"}}, "meta": map[string]any{"resourceType": "User"},
		"name": map[string]string{"formatted": "Entra Viewer", "familyName": "Viewer", "givenName": "Entra"}, "roles": []any{},
		enterpriseSchema: map[string]any{"department": "Media", "employeeNumber": "123", "manager": map[string]string{"value": "directory-manager"}},
	})
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"userName":"entra@example.com"`) {
		t.Fatalf("Entra SCIM create = %d %q", created.Code, created.Body.String())
	}
	roleClaim := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{
		"schemas": []string{scimUserSchema, enterpriseSchema}, "userName": "unsupported-role@example.com", "roles": []map[string]string{{"value": "admin"}},
	})
	lookup := scimCall(t, handler, scimTestToken, http.MethodGet, `/scim/v2/Users?filter=userName%20eq%20%22unsupported-role%40example.com%22`, nil)
	if roleClaim.Code != http.StatusCreated || !strings.Contains(lookup.Body.String(), `"totalResults":1`) {
		t.Fatalf("ignored Entra roles = %d lookup=%d %q", roleClaim.Code, lookup.Code, lookup.Body.String())
	}
	for name, extension := range map[string]any{
		"unknown":           map[string]string{"privilege": "owner"},
		"oversized-manager": map[string]string{"manager": strings.Repeat("m", 257)},
	} {
		userName := name + "@example.com"
		response := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema, enterpriseSchema}, "userName": userName, enterpriseSchema: extension})
		unchanged := scimCall(t, handler, scimTestToken, http.MethodGet, `/scim/v2/Users?filter=userName%20eq%20%22`+userName+`%22`, nil)
		if response.Code != http.StatusBadRequest || !strings.Contains(unchanged.Body.String(), `"totalResults":0`) {
			t.Fatalf("invalid Entra extension %s = %d lookup=%d %q", name, response.Code, unchanged.Code, unchanged.Body.String())
		}
	}
}

const scimEnterpriseSchema = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
