package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCoverageJSONCompositeShapes(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		target reflect.Type
		valid  bool
	}{
		{"nil target", "value", nil, false},
		{"map values", map[string]any{"key": "value"}, reflect.TypeFor[map[string]string](), true},
		{"null map value", map[string]any{"key": nil}, reflect.TypeFor[map[string]string](), false},
		{"non map", "value", reflect.TypeFor[map[string]string](), true},
		{"array", []any{"value"}, reflect.TypeFor[[1]string](), true},
		{"null element", []any{nil}, reflect.TypeFor[[]string](), false},
		{"non array", "value", reflect.TypeFor[[]string](), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesJSONShape(test.value, test.target); got != test.valid {
				t.Fatalf("shape = %v", got)
			}
		})
	}
}

func TestCoverageLoginLimiterCleanup(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Now()
	limiter.now = func() time.Time { return now }
	for index := 0; index <= maxLoginSources; index++ {
		limiter.sources[string(rune(index))] = loginBucket{until: now.Add(-time.Second)}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.RemoteAddr = "local"
	if allowed, _ := limiter.allow(request, "Owner"); !allowed || len(limiter.sources) != 1 {
		t.Fatalf("cleanup left %d sources", len(limiter.sources))
	}
	if source := loginSource(request, strings.Repeat("A", 90)); source != "local|"+strings.Repeat("a", 80) {
		t.Fatalf("bounded source = %q", source)
	}
}
