package servertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// PostRouteLoginCookie preserves the original form encoding, address, and cookie checks.
func PostRouteLoginCookie(t *testing.T, handler http.Handler, form url.Values, remoteAddress string) *http.Cookie {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = remoteAddress
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "__Host-kinosail_session" && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("web login = %d %q headers=%v", response.Code, response.Body.String(), response.Header())
	return nil
}

// ExerciseRouteWithCookie sends the original concrete route and cancels event streams after flush.
func ExerciseRouteWithCookie(t *testing.T, handler http.Handler, pattern string, cookie *http.Cookie, concretePath func(string) string, afterFlush func(*httptest.ResponseRecorder, context.CancelFunc) http.ResponseWriter) *httptest.ResponseRecorder {
	t.Helper()
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		t.Fatalf("invalid route pattern %q", pattern)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, method, concretePath(path), strings.NewReader("{}"))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	writer := http.ResponseWriter(response)
	if pattern == "GET /api/v1/events" {
		writer = afterFlush(response, cancel)
	}
	handler.ServeHTTP(writer, request)
	return response
}

// RouteAuthorizationServer retains enrollment secrets beside the real app handler.
type RouteAuthorizationServer struct {
	http.Handler
	OwnerSecret, ViewerSecret string
}

// LoginRouteCookie preserves factor enrollment and cookie login for the route test server.
func LoginRouteCookie(t *testing.T, handler http.Handler, name, password string, loginProfile func(*testing.T, http.Handler, string, string) string, nextAddress func() string) *http.Cookie {
	t.Helper()
	form := url.Values{"name": {name}, "password": {password}}
	if secured, ok := handler.(*RouteAuthorizationServer); ok && name == "Owner" {
		form.Set("code", CurrentTOTP(secured.OwnerSecret))
	} else if secured, ok := handler.(*RouteAuthorizationServer); ok && name == "Viewer" {
		if secured.ViewerSecret == "" {
			loginProfile(t, handler, name, password)
		}
		form.Set("code", CurrentTOTP(secured.ViewerSecret))
	}
	return PostRouteLoginCookie(t, handler, form, nextAddress())
}
