package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLocalModeAcceptsPrivateNetworkAddressesFromPublishedPorts(t *testing.T) {
	t.Parallel()
	for _, origin := range []string{"https://localhost:38127", "http://localhost:38127", "https://127.0.0.1:38127"} {
		handler := server.New(server.Config{MediaDir: t.TempDir(), DataDir: t.TempDir(), AuthURL: origin})
		for _, host := range []string{"192.168.1.20:38127", "10.0.0.5:8080", "172.20.0.9:38127", "100.101.102.103:38127", "[fd00::5]:38127", "169.254.10.20:38127", "127.0.0.1:38127"} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/health", nil)
			request.Host = host
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code == http.StatusMisdirectedRequest {
				t.Fatalf("%s rejected private network address %q: %q", origin, host, response.Body.String())
			}
		}
	}
}

func TestLocalModeRejectsNamesAndPublicAddressesWithGuidance(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{MediaDir: t.TempDir(), DataDir: t.TempDir(), AuthURL: "https://localhost:38127"})
	for _, tc := range []struct{ host, language, want string }{
		{"rebind.attacker.example:38127", "en", "This address is not allowed. Open Kinosail at its configured address, or add this address to KINOSAIL_TLS_HOSTS."},
		{"203.0.113.9:38127", "en", "This address is not allowed. Open Kinosail at its configured address, or add this address to KINOSAIL_TLS_HOSTS."},
		{"rebind.attacker.example:38127", "zh-Hans", "KINOSAIL_TLS_HOSTS"},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.Host = tc.host
		request.Header.Set("Accept", "text/html")
		request.Header.Set("Accept-Language", tc.language)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		body := response.Body.String()
		if response.Code != http.StatusMisdirectedRequest || !strings.Contains(body, tc.want) || strings.Contains(body, "Request could not be completed.") || response.Header().Get("Set-Cookie") != "" {
			t.Fatalf("%s %s = %d %q", tc.host, tc.language, response.Code, body)
		}
		if tc.language == "zh-Hans" && (strings.Contains(body, "This address is not allowed") || !strings.Contains(body, "地址")) {
			t.Fatalf("Chinese rejection was not translated: %q", body)
		}
	}
}

func TestPublicHTTPSOriginStillRejectsUnlistedPrivateAddresses(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{MediaDir: t.TempDir(), DataDir: t.TempDir(), AuthURL: "https://player.example.com"})
	for _, host := range []string{"192.168.1.20:38127", "192.168.1.20", "[fd00::5]:443"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/health", nil)
		request.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusMisdirectedRequest {
			t.Fatalf("public HTTPS origin accepted unlisted address %q: %d", host, response.Code)
		}
	}
}
