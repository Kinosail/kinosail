package httpguard

import (
	"context"
	"net/http"
	"sync"
)

type routeMemoKey struct{}

type routeMemo struct {
	mu      sync.Mutex
	set     bool
	method  string
	host    string
	path    string
	rawPath string
	pattern string
}

func WithRouteMemo(ctx context.Context) context.Context {
	return context.WithValue(ctx, routeMemoKey{}, &routeMemo{})
}

func MemoizedRoutePattern(resolve func(*http.Request) string) func(*http.Request) string {
	return func(request *http.Request) string {
		memo, _ := request.Context().Value(routeMemoKey{}).(*routeMemo)
		if memo == nil {
			return resolve(request)
		}
		memo.mu.Lock()
		defer memo.mu.Unlock()
		if memo.set && memo.method == request.Method && memo.host == request.Host && memo.path == request.URL.Path && memo.rawPath == request.URL.RawPath {
			return memo.pattern
		}
		memo.pattern = resolve(request)
		memo.set, memo.method, memo.host, memo.path, memo.rawPath = true, request.Method, request.Host, request.URL.Path, request.URL.RawPath
		return memo.pattern
	}
}

func NestedRoutePattern(root, application *http.ServeMux) func(*http.Request) string {
	return MemoizedRoutePattern(func(request *http.Request) string {
		_, pattern := root.Handler(request)
		if pattern == "/" {
			_, pattern = application.Handler(request)
		}
		return pattern
	})
}
