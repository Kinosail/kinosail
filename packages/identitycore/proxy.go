package identitycore

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type remoteRequestContextKey struct{}

const HostNotAllowedMessage = "This address is not allowed. Open Kinosail at its configured address, or add this address to KINOSAIL_TLS_HOSTS."

func Remote(next http.Handler) http.Handler {
	if next == nil {
		return unavailableHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Header.Del("X-Kinosail-Remote")
		next.ServeHTTP(writer, markRemote(request))
	})
}

func RemoteRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	remote, _ := request.Context().Value(remoteRequestContextKey{}).(bool)
	return remote
}

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

func AllowedHost(rawURL string, aliases []string, defaultPort string, next http.Handler, reject func(http.ResponseWriter, *http.Request, string, int)) http.Handler {
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
	localOrigin := loopbackHost(origin.Hostname())
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
		if !allowed && localOrigin {
			allowed = privateAddressHost(actual)
		}
		if !allowed {
			slog.WarnContext(request.Context(), "Request host rejected", "operation", "host_check", "outcome", "rejected", "host_class", hostClass(actual), "local_origin", localOrigin)
			reject(writer, request, HostNotAllowedMessage, http.StatusMisdirectedRequest)
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

func privateAddressHost(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	address := net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"))
	return address != nil && (address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || cgnatIP(address))
}

func hostClass(host string) string {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	switch address := net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")); {
	case address == nil:
		return "name"
	case privateAddressHost(address.String()):
		return "private_address"
	default:
		return "public_address"
	}
}
