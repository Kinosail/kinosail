package httpguard

import "net/http"

func NestedRoutePattern(root, application *http.ServeMux) func(*http.Request) string {
	return func(request *http.Request) string {
		_, pattern := root.Handler(request)
		if pattern == "/" {
			_, pattern = application.Handler(request)
		}
		return pattern
	}
}
