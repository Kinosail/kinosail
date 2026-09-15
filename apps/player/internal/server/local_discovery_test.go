package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/serverdiscovery"
)

func TestLocalDiscoveryHostKeepsExplicitAllowlist(t *testing.T) {
	origin := "http://localhost:38127"
	alias := serverdiscovery.LoopbackAlias(origin)
	if alias == "" {
		t.Skip("operating-system hostname cannot be advertised")
	}
	for _, tc := range []struct {
		origin, host string
		allowed      bool
	}{
		{origin, alias + ":38127", true},
		{origin, "unrelated.local:38127", false},
		{origin, alias + ":9999", false},
		{"https://player.example.com", alias + ":38127", false},
		{"https://player.example.com", "player.example.com", true},
	} {
		called := false
		handler := allowedHost(tc.origin, nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+tc.host+"/", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if called != tc.allowed {
			t.Errorf("%+v: handler called=%v", tc, called)
		}
		if !tc.allowed && response.Code != http.StatusMisdirectedRequest {
			t.Errorf("unexpected rejection: %d", response.Code)
		}
	}
}

func TestAndroidDiscoveredInterfaceAddressUsesNormalHostGate(t *testing.T) {
	origin := "http://localhost:38127"
	aliases := serverdiscovery.LocalAliases(origin)
	for _, alias := range aliases {
		host, _, err := net.SplitHostPort(alias)
		if err != nil || net.ParseIP(host) == nil {
			continue
		}
		for _, tc := range []struct {
			origin, host string
			allowed      bool
		}{
			{origin, alias, true},
			{origin, net.JoinHostPort(host, "9999"), false},
			{"https://player.example.com", alias, false},
		} {
			called := false
			handler := allowedHost(tc.origin, nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true }))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+tc.host+"/", nil))
			if called != tc.allowed {
				t.Fatalf("%+v: called=%v", tc, called)
			}
		}
		return
	}
	t.Skip("no private interface address is available")
}

func TestDiscoveryHostnameWorksOnImplicitHTTPAndHTTPSPorts(t *testing.T) {
	for _, origin := range []string{"http://localhost", "https://localhost"} {
		alias := serverdiscovery.LoopbackAlias(origin)
		if alias == "" {
			t.Skip("hostname cannot be advertised")
		}
		called := false
		handler := allowedHost(origin, nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true }))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+alias+"/", nil))
		if !called {
			t.Fatalf("discovered default-port hostname rejected: %s", origin)
		}
	}
}
