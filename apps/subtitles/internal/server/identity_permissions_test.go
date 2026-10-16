package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requestWithCookieRequest(t *testing.T, method, path, body string, cookie *http.Cookie) *http.Request {
	t.Helper()
	request := requestWithBody(t, method, path, body)
	request.AddCookie(cookie)
	return request
}

func requestWithBody(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, "http://kinosail.test"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func serveRequest(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
