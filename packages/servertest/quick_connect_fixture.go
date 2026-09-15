package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// QuickConnectFixture uses the real app constructor and established request helpers.
type QuickConnectFixture struct {
	New       func(string, time.Duration) http.Handler
	SignIn    func(*testing.T, http.Handler, string, string) *http.Cookie
	AddViewer func(*testing.T, http.Handler, *http.Cookie)
	Web       AuthCookieRequest
	APIKey    func(*testing.T, http.Handler, string, string) *httptest.ResponseRecorder
	API       func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
}

// QuickConnectResponse is the app's code, polling secret and approved token response.
type QuickConnectResponse struct{ Code, Secret, Token string }

// QuickConnect decodes the original form request response.
func QuickConnect(t *testing.T, handler http.Handler, path, body string) QuickConnectResponse {
	t.Helper()
	response := QuickConnectRecorder(t, handler, path, body)
	var value QuickConnectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("Quick Connect = %d %q", response.Code, response.Body.String())
	}
	return value
}

// QuickConnectRecorder sends the original form request.
func QuickConnectRecorder(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
