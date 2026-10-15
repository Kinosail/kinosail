package server_test

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestPublicInternetListenerDeniesSetupOwnersAndUnsecuredPasswordLogin(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	ownerRequest := requestWithCookieRequest(t, http.MethodGet, "/settings", "", owner)
	if response := serveRequest(server.Remote(handler), ownerRequest); response.Code != http.StatusForbidden {
		t.Fatalf("remote Owner settings = %d %q", response.Code, response.Body.String())
	}
	if response := serveRequest(server.Remote(handler), requestWithBody(t, http.MethodGet, "/setup", "")); response.Code != http.StatusNotFound {
		t.Fatalf("remote setup = %d %q", response.Code, response.Body.String())
	}
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	login := requestWithBody(t, http.MethodPost, "/login", "name=Viewer&password=viewer-password")
	if response := serveRequest(server.Remote(handler), login); response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "passkey or Quick Connect") {
		t.Fatalf("remote password login = %d %q", response.Code, response.Body.String())
	}
}

func TestPublicPasskeyOriginKeepsExplicitLANAliasesAvailable(t *testing.T) {
	t.Parallel()
	configured, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		values := map[string]string{"KINOSAIL_AUTH_URL": "https://family-media.duckdns.org", "KINOSAIL_TLS_HOSTS": `["media.home","192.0.2.10"]`}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://family-media.duckdns.org", Configuration: configured})
	for _, host := range []string{"media.home:38128", "192.0.2.10:38128"} {
		request := requestWithBody(t, http.MethodGet, "/login", "")
		request.Host = host
		if response := serveRequest(handler, request); response.Code != http.StatusOK {
			t.Fatalf("LAN alias %s = %d %q", host, response.Code, response.Body.String())
		}
	}
}

func TestPublicHTTPSFailsClosedWhenAuthorizationStateCannotLoad(t *testing.T) {
	t.Parallel()
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("k", 32), Listen: "127.0.0.1:8443", DataDir: t.TempDir()}, remoteaccess.Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	invalidData := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(invalidData, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: invalidData, RequireAuth: true, InternetAccess: manager})
	if response := serveRequest(handler, requestWithBody(t, http.MethodGet, "/", "")); response.Code != http.StatusServiceUnavailable || manager.Status().State != "killed" {
		t.Fatalf("failed authorization state = %d status=%#v", response.Code, manager.Status())
	}
}

func TestPublicRangeRequestsRejectAmbiguousAndOverflowingValues(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	for _, value := range []string{"bytes=0-1,2-3", "bytes=18446744073709551616-", "items=0-1", "bytes=-"} {
		request := requestWithBody(t, http.MethodGet, "/media/missing", "")
		request.Header.Set("Range", value)
		if response := serveRequest(server.Remote(handler), request); response.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("public Range %q = %d %q", value, response.Code, response.Body.String())
		}
	}
	local := requestWithBody(t, http.MethodGet, "/media/missing", "")
	local.Header.Set("Range", "bytes=0-1,2-3")
	if response := serveRequest(handler, local); response.Code == http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("public range policy changed LAN response: %d %q", response.Code, response.Body.String())
	}
}
