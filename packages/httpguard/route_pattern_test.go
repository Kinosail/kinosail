package httpguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNestedRoutePatternPreservesRootAndApplicationRoutes(t *testing.T) {
	root, application := http.NewServeMux(), http.NewServeMux()
	root.Handle("GET /discovery", http.NotFoundHandler())
	application.Handle("GET /items/{id}", http.NotFoundHandler())
	root.Handle("/", application)
	resolve := NestedRoutePattern(root, application)
	for path, expected := range map[string]string{"/discovery": "GET /discovery", "/items/one": "GET /items/{id}", "/missing": ""} {
		if got := resolve(httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)); got != expected {
			t.Errorf("%s: got %q, want %q", path, got, expected)
		}
	}
}
