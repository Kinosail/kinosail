package httpguard

import "net/http"

// NestedRoutePattern resolves a catch-all delegation to the application's route.
func NestedRoutePattern(root, application *http.ServeMux) func(*http.Request) string {
	return func(request *http.Request) string {
		_, pattern := root.Handler(request)
		if pattern == "/" {
			_, pattern = application.Handler(request)
		}
		return pattern
	}
}
