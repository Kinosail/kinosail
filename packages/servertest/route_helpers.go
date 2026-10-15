package servertest

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func CurrentTOTP(secret string) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	return totpForCounter(key, nextTOTPCounter(secret))
}

func RouteJSON(t *testing.T, handler http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(body); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, &payload)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func CreateRouteAPIKey(t *testing.T, handler http.Handler, owner, scope string) string {
	t.Helper()
	response := RouteJSON(t, handler, owner, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": scope + " route test", "scopes": scope})
	var result struct {
		Secret string `json:"secret"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Secret == "" {
		t.Fatalf("create %s API key = %d %q", scope, response.Code, response.Body.String())
	}
	return result.Secret
}
