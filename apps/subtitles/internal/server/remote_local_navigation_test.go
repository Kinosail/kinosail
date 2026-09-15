package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestPublicHTTPSKeepsLANBrowserRecoveryLocal(t *testing.T) {
	t.Parallel()
	const origin = "https://family-media.duckdns.org"
	configured, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		values := map[string]string{"KINOSAIL_AUTH_URL": origin, "KINOSAIL_TLS_HOSTS": `["media.home"]`}
		value, found := values[key]
		return value, found
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("t", 32), Listen: "127.0.0.1:8443", DataDir: t.TempDir()}, remoteaccess.Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: origin, Configuration: configured, InternetAccess: manager})
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequestWithContext(t.Context(), method, "https://media.home:38128/settings", nil)
		request.Header.Set("Accept", "text/html")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/setup" {
			t.Fatalf("local %s recovery redirected to public ingress: %d %q", method, response.Code, response.Header().Get("Location"))
		}
		remote := httptest.NewRecorder()
		publicRequest := httptest.NewRequestWithContext(t.Context(), method, origin+"/setup", nil)
		publicRequest.Header.Set("Accept", "text/html")
		Remote(handler).ServeHTTP(remote, publicRequest)
		if remote.Code != http.StatusNotFound {
			t.Fatalf("public setup = %d", remote.Code)
		}
	}
	// Disabling page redirects does not relax WebAuthn's exact-origin check.
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://media.home:38128/api/v1/passkeys/login/begin", strings.NewReader("{}"))
	request.Header.Set("Origin", "https://media.home:38128")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("wrong-origin passkey ceremony = %d %q", response.Code, response.Body.String())
	}
}
