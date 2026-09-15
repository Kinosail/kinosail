package scim

import (
	"net/http"
)

func (api *scimAPI) serviceProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if err := scimDiscoveryQuery(request); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	writeSCIMJSON(writer, map[string]any{
		"schemas":               []string{scimServiceSchema},
		"patch":                 map[string]bool{"supported": true},
		"bulk":                  map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":                map[string]any{"supported": true, "maxResults": maxSCIMPageSize},
		"changePassword":        map[string]bool{"supported": false},
		"sort":                  map[string]bool{"supported": false},
		"etag":                  map[string]bool{"supported": true},
		"authenticationSchemes": []map[string]string{{"type": "oauthbearertoken", "name": "OAuth Bearer Token", "description": "SCIM bearer token"}},
	}, http.StatusOK)
}

func (api *scimAPI) resourceTypes(writer http.ResponseWriter, request *http.Request) {
	if err := scimDiscoveryQuery(request); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	writeSCIMJSON(writer, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 1, "startIndex": 1, "itemsPerPage": 1, "Resources": []map[string]any{scimResourceType()}}, http.StatusOK)
}

func (api *scimAPI) resourceType(writer http.ResponseWriter, request *http.Request) {
	if !scimDiscoveryResource(writer, request, "User") {
		return
	}
	writeSCIMJSON(writer, scimResourceType(), http.StatusOK)
}

func scimResourceType() map[string]any {
	return map[string]any{"schemas": []string{scimResourceSchema}, "id": "User", "name": "User", "endpoint": "/scim/v2/Users", "schema": scimUserSchema, "schemaExtensions": []map[string]any{{"schema": scimEnterpriseUser, "required": false}}, "meta": map[string]string{"resourceType": "ResourceType", "location": "/scim/v2/ResourceTypes/User"}}
}

func (api *scimAPI) schemas(writer http.ResponseWriter, request *http.Request) {
	if err := scimDiscoveryQuery(request); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	writeSCIMJSON(writer, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 2, "startIndex": 1, "itemsPerPage": 2, "Resources": []map[string]any{scimUserSchemaValue(), scimEnterpriseSchemaValue()}}, http.StatusOK)
}

func (api *scimAPI) schema(writer http.ResponseWriter, request *http.Request) {
	if err := scimDiscoveryQuery(request); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	switch request.PathValue("id") {
	case scimUserSchema:
		writeSCIMJSON(writer, scimUserSchemaValue(), http.StatusOK)
	case scimEnterpriseUser:
		writeSCIMJSON(writer, scimEnterpriseSchemaValue(), http.StatusOK)
	default:
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM discovery resource was not found")
	}
}

func scimEnterpriseSchemaValue() map[string]any {
	attributes := make([]map[string]any, 0, 6)
	for _, name := range []string{"employeeNumber", "costCenter", "organization", "division", "department"} {
		attributes = append(attributes, map[string]any{"name": name, "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"})
	}
	attributes = append(attributes, map[string]any{"name": "manager", "type": "complex", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "subAttributes": []map[string]any{{"name": "value", "type": "string", "multiValued": false, "required": false, "caseExact": true, "mutability": "readWrite", "returned": "default"}, {"name": "$ref", "type": "reference", "referenceTypes": []string{"User"}, "multiValued": false, "required": false, "caseExact": true, "mutability": "readWrite", "returned": "default"}, {"name": "displayName", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readOnly", "returned": "default"}}})
	return map[string]any{"schemas": []string{scimSchemaSchema}, "id": scimEnterpriseUser, "name": "EnterpriseUser", "description": "Enterprise Viewer Profile attributes", "attributes": attributes}
}

func scimDiscoveryResource(writer http.ResponseWriter, request *http.Request, wanted string) bool {
	if err := scimDiscoveryQuery(request); err != nil {
		writeSCIMValidationError(writer, err)
		return false
	}
	if request.PathValue("id") != wanted {
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM discovery resource was not found")
		return false
	}
	return true
}

func scimDiscoveryQuery(request *http.Request) error {
	_, err := scimQueryValues(request, "attributes", "excludedAttributes", "filter", "startIndex", "count", "sortBy", "sortOrder")
	return err
}

func scimUserSchemaValue() map[string]any {
	return map[string]any{
		"schemas": []string{scimSchemaSchema}, "id": scimUserSchema, "name": "User", "description": "Kinosail Viewer Profile", "attributes": []map[string]any{
			{"name": "userName", "type": "string", "multiValued": false, "required": true, "caseExact": false, "uniqueness": "server", "mutability": "readWrite", "returned": "default"},
			{"name": "externalId", "type": "string", "multiValued": false, "required": false, "caseExact": true, "uniqueness": "none", "mutability": "readWrite", "returned": "default"},
			{"name": "active", "type": "boolean", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"},
			{"name": "displayName", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"},
			{"name": "name", "type": "complex", "multiValued": false, "required": false, "mutability": "readWrite", "returned": "default", "subAttributes": []map[string]any{{"name": "formatted", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"}, {"name": "givenName", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"}, {"name": "familyName", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"}}},
			{"name": "emails", "type": "complex", "multiValued": true, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default", "subAttributes": []map[string]any{{"name": "value", "type": "string", "multiValued": false, "required": true, "caseExact": false, "mutability": "readWrite", "returned": "default"}, {"name": "type", "type": "string", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"}, {"name": "primary", "type": "boolean", "multiValued": false, "required": false, "caseExact": false, "mutability": "readWrite", "returned": "default"}}},
		},
	}
}
