package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type cancelAfterFlushWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (writer *cancelAfterFlushWriter) Flush() {
	writer.ResponseRecorder.Flush()
	writer.cancel()
}

func exerciseRoute(t *testing.T, handler http.Handler, pattern, token string, capability bool) *httptest.ResponseRecorder {
	t.Helper()
	return exerciseRouteWithHeaders(t, handler, pattern, token, capability, nil)
}

func exerciseRouteWithHeaders(t *testing.T, handler http.Handler, pattern, token string, capability bool, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return exerciseRouteAsMethod(t, handler, pattern, "", token, capability, headers)
}

func exerciseRouteAsMethod(t *testing.T, handler http.Handler, pattern, requestMethod, token string, capability bool, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		t.Fatalf("invalid route pattern %q", pattern)
	}
	if requestMethod != "" {
		method = requestMethod
	}
	path = concreteRoutePath(path)
	if capability && strings.HasPrefix(pattern, "GET /Videos/") || capability && pattern == "GET /Audio/{id}/{stream}" {
		path += "?playSessionId=invalid"
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader("{}"))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	writer := http.ResponseWriter(response)
	if pattern == "GET /api/v1/events" {
		writer = &cancelAfterFlushWriter{response, cancel}
	}
	handler.ServeHTTP(writer, request)
	return response
}
