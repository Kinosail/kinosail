package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type QuickConnectFixture struct {
	New       func(string, time.Duration) http.Handler
	SignIn    func(*testing.T, http.Handler, string, string) *http.Cookie
	AddViewer func(*testing.T, http.Handler, *http.Cookie)
	Web       AuthCookieRequest
	APIKey    func(*testing.T, http.Handler, string, string) *httptest.ResponseRecorder
	API       func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
}

type QuickConnectResponse struct{ Code, Secret, Token string }

func QuickConnect(t *testing.T, handler http.Handler, path, body string) QuickConnectResponse {
	t.Helper()
	response := QuickConnectRecorder(t, handler, path, body)
	var value QuickConnectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("Quick Connect = %d %q", response.Code, response.Body.String())
	}
	return value
}

func QuickConnectRecorder(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
