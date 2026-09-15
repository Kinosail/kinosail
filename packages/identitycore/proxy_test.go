package identitycore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteMarksRequestAndRemovesSpoofedHeader(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Kinosail-Remote", "spoofed")
	response := httptest.NewRecorder()
	Remote(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if !RemoteRequest(request) || request.Header.Get("X-Kinosail-Remote") != "" {
			t.Fatal("public listener marker was not isolated from caller headers")
		}
	})).ServeHTTP(response, request)
	if RemoteRequest(nil) {
		t.Fatal("nil request was remote")
	}
	if response := httptest.NewRecorder(); true {
		Remote(nil).ServeHTTP(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("nil next response = %d", response.Code)
		}
	}
}

func TestTrustedProxyMarksAuthenticatedRequest(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Kinosail-Proxy-Token", "proxy-capability")
	request.Header.Set("X-Forwarded-For", "203.0.113.8, 192.0.2.1")
	request.Header.Set("X-Forwarded-Host", "public.example")
	TrustedProxy("proxy-capability", http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if !RemoteRequest(request) || request.RemoteAddr != "203.0.113.8:0" || request.Header.Get("X-Kinosail-Remote") != "true" {
			t.Fatalf("trusted proxy request = remote %v, address %q, marker %q", RemoteRequest(request), request.RemoteAddr, request.Header.Get("X-Kinosail-Remote"))
		}
		if request.Header.Get("X-Kinosail-Proxy-Token") != "" || request.Header.Get("X-Forwarded-Host") != "public.example" {
			t.Fatal("trusted proxy headers were mishandled")
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
}

func TestTrustedProxyRequiresNextHandler(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	TrustedProxy("token", nil).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("nil next response = %d", response.Code)
	}
}

func TestTrustedProxyRejectsUntrustedInputsBeforeRemoteEffects(t *testing.T) {
	t.Parallel()
	for name, token := range map[string]string{"missing": "", "wrong": "wrong", "oversized config": strings.Repeat("x", 1025)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			request.Header.Set("X-Kinosail-Proxy-Token", "proxy-capability")
			request.Header.Set("X-Forwarded-For", "203.0.113.8")
			request.Header.Set("Forwarded", "for=203.0.113.8")
			request.Header.Set("X-Kinosail-Remote", "spoofed")
			TrustedProxy(token, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				if RemoteRequest(request) || request.Header.Get("X-Kinosail-Proxy-Token") != "" || request.Header.Get("X-Forwarded-For") != "" || request.Header.Get("Forwarded") != "" || request.Header.Get("X-Kinosail-Remote") != "" {
					t.Fatal("untrusted forwarding state reached the application")
				}
			})).ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func TestTrustedProxyAcceptsExactTokenBound(t *testing.T) {
	t.Parallel()
	token := strings.Repeat("x", 1024)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Kinosail-Proxy-Token", token)
	TrustedProxy(token, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if !RemoteRequest(request) {
			t.Fatal("maximum-length proxy token was rejected")
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
	empty := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	TrustedProxy("", http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if RemoteRequest(request) {
			t.Fatal("empty proxy token was trusted")
		}
	})).ServeHTTP(httptest.NewRecorder(), empty)
}

func TestTrustedProxyPreservesRemoteListenerWithoutTrustingForwardedHeaders(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	Remote(TrustedProxy("proxy-capability", http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if !RemoteRequest(request) || request.Header.Get("X-Kinosail-Remote") != "true" || request.Header.Get("X-Forwarded-For") != "" {
			t.Fatal("dedicated public listener state was not preserved and sanitized")
		}
	}))).ServeHTTP(httptest.NewRecorder(), request)
}

func TestAllowedHost(t *testing.T) { //nolint:cyclop,gocognit // One host-policy fixture covers origin, alias, loopback, and invalid configuration paths.
	t.Parallel()
	reject := func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	}
	called := 0
	handler := AllowedHost("https://player.example", []string{"kino.local", "localhost"}, "38127", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ }), reject)
	for _, host := range []string{"player.example", "kino.local", "kino.local:38127", "localhost:38127", "127.0.0.1:38127"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("host %q response = %d", host, response.Code)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Host = "attacker.example"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || called != 5 {
		t.Fatalf("disallowed host response=%d called=%d", response.Code, called)
	}
	for _, rawURL := range []string{"", "://bad", "https://user@example.com", "https://example.com/path", "https://example.com?query=1", "https://example.com#fragment"} {
		response = httptest.NewRecorder()
		AllowedHost(rawURL, nil, "38127", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), reject).ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("inactive host policy %q response = %d", rawURL, response.Code)
		}
	}
	for name, handler := range map[string]http.Handler{
		"nil next":   AllowedHost("https://example.com", nil, "38127", nil, reject),
		"nil reject": AllowedHost("https://example.com", nil, "38127", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil),
		"empty port": AllowedHost("https://example.com", nil, "", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), reject),
		"long port":  AllowedHost("https://example.com", nil, "123456", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), reject),
	} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("invalid host config %s response = %d", name, response.Code)
		}
	}
	if !sameLoopbackHost("127.0.0.1:1", "localhost:2") || sameLoopbackHost("127.0.0.1", "localhost:2") || sameLoopbackHost("127.0.0.1:1", "localhost") || sameLoopbackHost("192.0.2.1:1", "localhost:2") {
		t.Fatal("loopback host equivalence changed")
	}
	for host, loopback := range map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "192.0.2.1": false, "invalid": false} {
		if loopbackHost(host) != loopback {
			t.Fatalf("loopbackHost(%q) = %v", host, loopbackHost(host))
		}
	}
}
