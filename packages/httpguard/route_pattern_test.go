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

func TestMemoizedRoutePatternResolvesOncePerRequestRoute(t *testing.T) {
	application := http.NewServeMux()
	application.Handle("GET /items/{id}", http.NotFoundHandler())
	application.Handle("POST /items/{id}", http.NotFoundHandler())
	application.Handle("GET /settings", http.NotFoundHandler())
	lookups := 0
	resolve := MemoizedRoutePattern(func(request *http.Request) string {
		lookups++
		_, pattern := application.Handler(request)
		return pattern
	})
	request := httptest.NewRequestWithContext(WithRouteMemo(t.Context()), http.MethodGet, "/items/one", nil)
	for range 3 {
		if got := resolve(request); got != "GET /items/{id}" {
			t.Fatalf("memoized pattern = %q", got)
		}
	}
	rewritten := request.Clone(request.Context())
	rewritten.URL.Path = "/settings"
	posted := request.Clone(request.Context())
	posted.Method = http.MethodPost
	if resolve(rewritten) != "GET /settings" || resolve(posted) != "POST /items/{id}" || resolve(request) != "GET /items/{id}" || lookups != 4 {
		t.Fatalf("rewritten request reused a stale pattern; lookups = %d", lookups)
	}
	if resolve(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)) != "GET /settings" || lookups != 5 {
		t.Fatal("request without a memo did not resolve directly")
	}
}
