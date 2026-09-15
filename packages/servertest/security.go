package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type SecurityConfig[Snapshot any] struct {
	DataDir, ProxyToken, AuthURL string
	Configuration                Snapshot
}

type securityContract[Snapshot any] struct {
	New             func(SecurityConfig[Snapshot]) http.Handler
	Load            func(string, string, func(string) (string, bool)) (Snapshot, error)
	Port, WrongPort string
}

func HTTPSecurity[Snapshot any](t *testing.T, newHandler func(SecurityConfig[Snapshot]) http.Handler, load func(string, string, func(string) (string, bool)) (Snapshot, error), port, wrongPort string) {
	t.Helper()
	security := securityContract[Snapshot]{newHandler, load, port, wrongPort}
	t.Run("PublicHTTPSResponsesAreSecureBehindProxy", security.testPublicHTTPSResponsesAreSecureBehindProxy)
	t.Run("UntrustedClientsCannotSpoofProxyHeaders", security.testUntrustedClientsCannotSpoofProxyHeaders)
	t.Run("ConfiguredPublicOriginIsAlsoTheHostAllowlist", security.testConfiguredPublicOriginIsAlsoTheHostAllowlist)
	t.Run("LocalhostHostAllowlistAcceptsEquivalentLoopbackAddresses", security.testLocalhostHostAllowlistAcceptsEquivalentLoopbackAddresses)
	t.Run("CrossOriginChangesAreDenied", security.testCrossOriginChangesAreDenied)
	t.Run("BrowserCookieChangesRequireOriginEvidence", security.testBrowserCookieChangesRequireOriginEvidence)
	t.Run("PublicBrowserCookieChangesRequireExactOriginEvidence", security.testPublicBrowserCookieChangesRequireExactOriginEvidence)
	t.Run("OpaqueOriginFromSameOriginBrowserNavigationIsAccepted", security.testOpaqueOriginFromSameOriginBrowserNavigationIsAccepted)
	t.Run("BrowserSecurityIsAutomatic", security.testBrowserSecurityIsAutomatic)
	t.Run("OversizedChangesAreRejected", security.testOversizedChangesAreRejected)
}

func (security securityContract[Snapshot]) testPublicHTTPSResponsesAreSecureBehindProxy(t *testing.T) {
	t.Parallel()

	handler := security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir(), ProxyToken: "proxy-capability"})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.test/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Kinosail-Proxy-Token", "proxy-capability")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || response.Header().Get("Strict-Transport-Security") == "" || len(response.Result().Cookies()) != 0 {
		t.Fatalf("headers = %v, cookies = %v", response.Header(), response.Result().Cookies())
	}
}

func (security securityContract[Snapshot]) testUntrustedClientsCannotSpoofProxyHeaders(t *testing.T) {
	t.Parallel()
	handler := security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir(), ProxyToken: "proxy-capability"})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/healthz", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Kinosail-Remote", "true")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("untrusted proxy headers accepted: %v", response.Header())
	}
}

func (security securityContract[Snapshot]) testConfiguredPublicOriginIsAlsoTheHostAllowlist(t *testing.T) {
	t.Parallel()
	configured, err := security.Load(t.TempDir(), "", func(name string) (string, bool) {
		return "[\"media.example\",\"192.0.2.55\"]", name == "KINOSAIL_TLS_HOSTS"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir(), AuthURL: "https://media.example:" + security.Port, Configuration: configured})
	for host, want := range map[string]int{"media.example:" + security.Port: http.StatusOK, "192.0.2.55:" + security.Port: http.StatusOK, "192.0.2.55:" + security.WrongPort: http.StatusMisdirectedRequest, "attacker.test:" + security.Port: http.StatusMisdirectedRequest} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+host+"/healthz", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("host %q = %d %q", host, response.Code, response.Body.String())
		}
	}
}

func (security securityContract[Snapshot]) testLocalhostHostAllowlistAcceptsEquivalentLoopbackAddresses(t *testing.T) {
	t.Parallel()
	handler := security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir(), AuthURL: "https://localhost:8443"})
	for _, host := range []string{"localhost:8443", "127.0.0.1:8443", "[::1]:8443", "127.0.0.1:49152"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+host+"/healthz", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("host %q = %d %q", host, response.Code, response.Body.String())
		}
	}
}

func (security securityContract[Snapshot]) testCrossOriginChangesAreDenied(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://kinosail.test/setup", strings.NewReader("name=Attacker&password=attacker-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://attacker.test")
	response := httptest.NewRecorder()
	security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir()}).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin setup = %d", response.Code)
	}
}

func (security securityContract[Snapshot]) testBrowserCookieChangesRequireOriginEvidence(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://kinosail.test/logout", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "stolen", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	response := httptest.NewRecorder()
	security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir()}).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("originless browser mutation = %d %q", response.Code, response.Body.String())
	}
}

func (security securityContract[Snapshot]) testPublicBrowserCookieChangesRequireExactOriginEvidence(t *testing.T) {
	t.Parallel()
	for name, origin := range map[string]string{
		"missing":      "",
		"wrong scheme": "http://kinosail.test",
		"wrong port":   "https://kinosail.test:8443",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/logout", nil)
			request.Header.Set("User-Agent", "Mozilla/5.0")
			if origin != "" {
				request.Header.Set("Origin", origin)
			}
			request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "stolen", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
			response := httptest.NewRecorder()
			security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir(), AuthURL: "https://kinosail.test"}).ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("public browser mutation = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func (security securityContract[Snapshot]) testOpaqueOriginFromSameOriginBrowserNavigationIsAccepted(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "null")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response := httptest.NewRecorder()
	security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir()}).ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("same-origin opaque navigation = %d %q", response.Code, response.Body.String())
	}
}

func (security securityContract[Snapshot]) testBrowserSecurityIsAutomatic(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir()}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil))
	headers := response.Header()
	if headers.Get("Cache-Control") != "no-store" || headers.Get("Cross-Origin-Opener-Policy") != "same-origin" || headers.Get("Cross-Origin-Resource-Policy") != "same-origin" || !strings.Contains(headers.Get("Content-Security-Policy"), "worker-src 'self' blob:") || strings.Contains(headers.Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatalf("security headers = %v", headers)
	}
}

func (security securityContract[Snapshot]) testOversizedChangesAreRejected(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader(strings.Repeat("x", 2<<20)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	security.New(SecurityConfig[Snapshot]{DataDir: t.TempDir()}).ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("oversized request = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}
