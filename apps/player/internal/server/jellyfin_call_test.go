package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func jellyfinCall(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Device="iPhone", DeviceId="test", Version="1.5", Token="`+token+`"`)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
