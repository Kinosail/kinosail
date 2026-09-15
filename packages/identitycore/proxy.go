package identitycore

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type remoteRequestContextKey struct{}

// Remote marks requests from a dedicated public listener.
func Remote(next http.Handler) http.Handler {
	if next == nil {
		return unavailableHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Header.Del("X-Kinosail-Remote")
		next.ServeHTTP(writer, markRemote(request))
	})
}

// RemoteRequest reports whether a trusted listener or proxy marked the request public.
func RemoteRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	remote, _ := request.Context().Value(remoteRequestContextKey{}).(bool)
	return remote
}

// TrustedProxy authenticates a reverse proxy and sanitizes forwarding headers.
func TrustedProxy(token string, next http.Handler) http.Handler {
	if next == nil {
		return unavailableHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		supplied := request.Header.Get("X-Kinosail-Proxy-Token")
		trusted := token != "" && len(token) <= 1024 && subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) == 1
		remote := RemoteRequest(request)
		request.Header.Del("X-Kinosail-Proxy-Token")
		if trusted || remote {
			request = markRemote(request)
			request.Header.Set("X-Kinosail-Remote", "true")
			if trusted {
				if client := forwardedClient(request); client != nil {
					request.RemoteAddr = net.JoinHostPort(client.String(), "0")
				}
			} else {
				stripForwardingHeaders(request)
			}
		} else {
			stripForwardingHeaders(request)
			request.Header.Del("X-Kinosail-Remote")
		}
		next.ServeHTTP(writer, request)
	})
}

// AllowedHost restricts requests to the canonical origin and configured aliases.
func AllowedHost(rawURL string, aliases []string, defaultPort string, next http.Handler, reject func(http.ResponseWriter, *http.Request, string, int)) http.Handler { //nolint:cyclop // Host validation stays explicit at this boundary.
	if next == nil || reject == nil || defaultPort == "" || len(defaultPort) > 5 {
		return unavailableHandler()
	}
	if rawURL == "" {
		return next
	}
	origin, err := url.Parse(rawURL)
	if err != nil || origin.Host == "" || origin.User != nil || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return next
	}
	expected := map[string]bool{strings.ToLower(origin.Host): true}
	port := origin.Port()
	if port == "" {
		port = defaultPort
	}
	for _, alias := range aliases {
		expected[strings.ToLower(alias)] = true
		expected[strings.ToLower(net.JoinHostPort(alias, port))] = true
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		actual, allowed := strings.ToLower(request.Host), false
		for host := range expected {
			allowed = allowed || actual == host || sameLoopbackHost(actual, host)
		}
		if !allowed {
			reject(writer, request, "request host is not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func unavailableHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "security middleware is unavailable", http.StatusInternalServerError)
	})
}

func markRemote(request *http.Request) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), remoteRequestContextKey{}, true))
}

func forwardedClient(request *http.Request) net.IP {
	first, _, _ := strings.Cut(request.Header.Get("X-Forwarded-For"), ",")
	return net.ParseIP(strings.TrimSpace(first))
}

func stripForwardingHeaders(request *http.Request) {
	request.Header.Del("Forwarded")
	request.Header.Del("X-Forwarded-For")
	request.Header.Del("X-Forwarded-Host")
	request.Header.Del("X-Forwarded-Proto")
}

func sameLoopbackHost(actual, expected string) bool {
	actualHost, _, actualErr := net.SplitHostPort(actual)
	expectedHost, _, expectedErr := net.SplitHostPort(expected)
	if actualErr != nil {
		return false
	}
	if expectedErr != nil {
		return false
	}
	if !loopbackHost(actualHost) {
		return false
	}
	return loopbackHost(expectedHost)
}

func loopbackHost(host string) bool {
	address := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || address != nil && address.IsLoopback()
}
