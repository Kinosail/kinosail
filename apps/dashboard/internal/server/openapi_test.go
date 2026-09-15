package server

import (
	"net/http"
	"strings"
	"testing"
)

func pathMethods() map[string][]string {
	return map[string][]string{
		"/api/v1":                          {"get"},
		"/api/v1/openapi.json":             {"get"},
		"/api/v1/setup":                    {"post"},
		"/api/v1/session":                  {"post", "delete"},
		"/api/v1/owner/password":           {"post"},
		"/api/v1/passkeys/register/begin":  {"post"},
		"/api/v1/passkeys/register/finish": {"post"},
		"/api/v1/passkeys/login/begin":     {"post"},
		"/api/v1/passkeys/login/finish":    {"post"},
		"/api/v1/mcp-token":                {"post", "delete"},
		"/api/v1/me":                       {"get"},
		"/api/v1/catalog":                  {"get"},
		"/api/v1/board":                    {"get", "put"},
		"/api/v1/apps":                     {"post"},
		"/api/v1/apps/{id}":                {"patch", "delete"},
		"/api/v1/apps/{id}/restore":        {"post"},
		"/api/v1/apps/order":               {"put"},
		"/api/v1/apps/{id}/check":          {"post"},
		"/api/v1/board/check":              {"post"},
		"/api/v1/export":                   {"get"},
		"/api/v1/import":                   {"put"},
		"/api/v1/import/preview":           {"post"},
		"/api/v1/import/external":          {"post"},
		"/api/v1/supporter":                {"get"},
		"/api/v1/supporter/activate":       {"post"},
	}
}

func TestOpenAPIInventoryAndRequiredContracts(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := app.request(t, http.MethodGet, "/api/v1/openapi.json", "", &session)
	if response.Code != http.StatusOK {
		t.Fatalf("OpenAPI status = %d, body = %s", response.Code, response.Body.String())
	}
	var document map[string]any
	decodeBody(t, response.Body, &document)
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi = %#v, want 3.1.0", document["openapi"])
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI paths is not an object")
	}
	assertOpenAPIPaths(t, paths)
	components, ok := document["components"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI components is not an object")
	}
	schemes, ok := components["securitySchemes"].(map[string]any)
	if !ok || schemes["cookieAuth"] == nil || schemes["bearerAuth"] == nil {
		t.Fatalf("OpenAPI security schemes = %#v", schemes)
	}
	security, ok := document["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatal("OpenAPI has no default authenticated security contract")
	}
}

func assertOpenAPIPaths(t *testing.T, paths map[string]any) {
	t.Helper()
	for path, methods := range pathMethods() {
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("OpenAPI omitted path %s", path)
			continue
		}
		for _, method := range methods {
			operation, ok := pathItem[method].(map[string]any)
			if !ok {
				t.Errorf("OpenAPI omitted %s %s", strings.ToUpper(method), path)
				continue
			}
			assertOpenAPIOperation(t, path, method, operation, pathItem)
		}
	}
}

func assertOpenAPIOperation(t *testing.T, path, method string, operation, pathItem map[string]any) {
	t.Helper()
	responses, ok := operation["responses"].(map[string]any)
	if !ok || len(responses) == 0 {
		t.Errorf("OpenAPI %s %s has no responses", strings.ToUpper(method), path)
	}
	if requiresRequestBody(method, path) {
		requestBody, ok := operation["requestBody"].(map[string]any)
		if !ok || requestBody["required"] != true {
			t.Errorf("OpenAPI %s %s has no required request body", strings.ToUpper(method), path)
		}
	}
	if strings.Contains(path, "{id}") && !hasRequiredIDParameter(operation, pathItem) {
		t.Errorf("OpenAPI %s %s has no required id path parameter", strings.ToUpper(method), path)
	}
}

func requiresRequestBody(method, path string) bool {
	return method != "get" && (path != "/api/v1/session" || method != "delete") && !strings.HasSuffix(path, "/begin")
}

func hasRequiredIDParameter(items ...map[string]any) bool {
	for _, item := range items {
		parameters, _ := item["parameters"].([]any)
		for _, raw := range parameters {
			parameter, _ := raw.(map[string]any)
			if parameter["$ref"] == "#/components/parameters/AppID" {
				return true
			}
			if parameter["name"] == "id" && parameter["in"] == "path" && parameter["required"] == true {
				return true
			}
		}
	}
	return false
}

func TestOpenAPIHasNoUndocumentedAPIRoutes(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := app.request(t, http.MethodGet, "/api/v1/openapi.json", "", &session)
	var document struct {
		Paths map[string]any `json:"paths"`
	}
	decodeBody(t, response.Body, &document)
	known := pathMethods()
	for path := range document.Paths {
		if _, ok := known[path]; !ok {
			t.Errorf("OpenAPI contains unknown API path %s", path)
		}
	}
}

func TestOpenAPIRequestSchemasMatchAcceptedJSON(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := app.request(t, http.MethodGet, "/api/v1/openapi.json", "", &session)
	var document map[string]any
	decodeBody(t, response.Body, &document)
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)

	assertCredentialProperties(t, schemas["Credentials"].(map[string]any))

	updateProperties := schemas["UpdateApp"].(map[string]any)["properties"].(map[string]any)
	if updateProperties["icon"] == nil {
		t.Error("UpdateApp omitted accepted icon property")
	}
	createRequired := schemas["CreateApp"].(map[string]any)["required"].([]any)
	for _, property := range createRequired {
		if property == "checkEnabled" {
			t.Error("CreateApp requires optional checkEnabled property")
		}
	}
	passwordChange := schemas["PasswordChange"].(map[string]any)
	if passwordChange["additionalProperties"] != false {
		t.Error("PasswordChange permits unknown properties")
	}
	passwordProperties := passwordChange["properties"].(map[string]any)
	if passwordProperties["currentPassword"] == nil || passwordProperties["newPassword"] == nil || passwordProperties["confirmPassword"] == nil {
		t.Errorf("PasswordChange properties = %#v", passwordProperties)
	}
	for _, property := range []string{"currentPassword", "newPassword", "confirmPassword"} {
		assertByteBoundedPasswordSchema(t, "PasswordChange."+property, passwordProperties[property].(map[string]any))
	}
}

func assertCredentialProperties(t *testing.T, credentials map[string]any) {
	t.Helper()
	credentialProperties := credentials["properties"].(map[string]any)
	expectedCredentials := map[string]bool{"name": true, "password": true, "device": true}
	for property := range credentialProperties {
		if !expectedCredentials[property] {
			t.Errorf("Credentials contains mis-cased or unknown property %q", property)
		}
		delete(expectedCredentials, property)
	}
	if len(expectedCredentials) != 0 {
		t.Errorf("Credentials omitted properties %#v", expectedCredentials)
	}
	assertByteBoundedPasswordSchema(t, "Credentials.password", credentialProperties["password"].(map[string]any))
}

func assertByteBoundedPasswordSchema(t *testing.T, name string, schema map[string]any) {
	t.Helper()
	if schema["minLength"] != nil || schema["maxLength"] != nil {
		t.Errorf("%s uses character bounds for a UTF-8 byte limit: %#v", name, schema)
	}
	if schema["description"] == nil {
		t.Errorf("%s does not describe its byte validation", name)
	}
}
