package httpguard

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func BenchmarkNestedRoutePatternThreeMiddlewareLookups(b *testing.B) {
	root, application := http.NewServeMux(), http.NewServeMux()
	for index := range 200 {
		application.Handle("GET /api/v1/resource"+strconv.Itoa(index)+"/{id}", http.NotFoundHandler())
	}
	root.Handle("GET /discovery", http.NotFoundHandler())
	root.Handle("/", application)
	resolve := NestedRoutePattern(root, application)
	for name, memoized := range map[string]bool{"without-memo": false, "with-memo": true} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/api/v1/resource150/item", nil)
				if memoized {
					request = request.WithContext(WithRouteMemo(request.Context()))
				}
				for range 3 {
					if resolve(request) == "" {
						b.Fatal("route was not resolved")
					}
				}
			}
		})
	}
}
