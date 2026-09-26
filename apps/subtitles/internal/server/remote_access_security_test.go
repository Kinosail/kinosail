package server_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
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

func TestPublicQuickConnectCannotInheritOwnerAccess(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	started := serveRequest(server.Remote(handler), requestWithBody(t, http.MethodPost, "/api/v1/quick-connect", "device=Internet+TV"))
	if started.Code != http.StatusCreated {
		t.Fatalf("start public Quick Connect = %d %q", started.Code, started.Body.String())
	}
	var state struct{ Code string }
	if err := json.Unmarshal(started.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	approved := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+state.Code, "", owner))
	if approved.Code != http.StatusForbidden {
		t.Fatalf("approve public Quick Connect as Owner = %d %q", approved.Code, approved.Body.String())
	}
}

func TestPublicInternetListenerAllowsOnlyRemoteEnabledViewers(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, ProxyToken: "proxy-capability"})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	disableTestMFA(t, handler, owner.Value)
	for _, input := range []string{
		"name=Remote&password=remote-password&rating=all&libraries=all&remote=true",
		"name=Local&password=local-password&rating=all&libraries=all",
	} {
		if response := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", input, owner)); response.Code != http.StatusSeeOther {
			t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
		}
	}
	remote := signInTestProfile(t, handler, "/login", "name=Remote&password=remote-password")
	local := signInTestProfile(t, handler, "/login", "name=Local&password=local-password")
	if response := serveRequest(server.Remote(handler), requestWithCookieRequest(t, http.MethodGet, "/", "", remote)); response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "public sign-in") {
		t.Fatalf("password-only public Viewer = %d %q", response.Code, response.Body.String())
	}
	enrollment := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", remote)
	confirmTestFactor(t, handler, remote, enrollment)
	if response := serveRequest(server.Remote(handler), requestWithCookieRequest(t, http.MethodGet, "/", "", remote)); response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "public sign-in") {
		t.Fatalf("LAN session reused publicly = %d %q", response.Code, response.Body.String())
	}
	if response := serveRequest(server.Remote(handler), requestWithCookieRequest(t, http.MethodGet, "/", "", local)); response.Code != http.StatusForbidden {
		t.Fatalf("local-only Viewer = %d %q", response.Code, response.Body.String())
	}
	direct := requestWithCookieRequest(t, http.MethodGet, "/", "", local)
	direct.Header.Set("X-Kinosail-Remote", "true")
	if response := serveRequest(handler, direct); response.Code != http.StatusOK {
		t.Fatalf("spoofed remote header changed local policy = %d %q", response.Code, response.Body.String())
	}
}

func TestPublicQuickConnectRequiresStrongRemoteEnabledViewer(t *testing.T) { //nolint:cyclop,funlen // One scenario proves the full public Quick Connect policy and revocation path.
	t.Parallel()
	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	disableTestMFA(t, handler, owner.Value)
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	viewer := signInTestProfile(t, handler, "/login", "name=Viewer&password=viewer-password")
	public := server.Remote(handler)
	start := func() (code, secret string) {
		response := serveRequest(public, requestWithBody(t, http.MethodPost, "/api/v1/quick-connect", "device=Internet+TV"))
		var state struct{ Code, Secret string }
		if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &state) != nil {
			t.Fatalf("start Quick Connect = %d %q", response.Code, response.Body.String())
		}
		return state.Code, state.Secret
	}
	code, _ := start()
	if response := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+code, "", viewer)); response.Code != http.StatusForbidden {
		t.Fatalf("password-only Quick Connect approval = %d %q", response.Code, response.Body.String())
	}
	enrollment := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", viewer)
	confirmTestFactor(t, handler, viewer, enrollment)
	code, secret := start()
	if response := serveRequest(public, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+code, "", viewer)); response.Code != http.StatusNotFound {
		t.Fatalf("public Quick Connect approval = %d %q", response.Code, response.Body.String())
	}
	if response := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+code, "", viewer)); response.Code != http.StatusNoContent {
		t.Fatalf("strong Quick Connect approval = %d %q", response.Code, response.Body.String())
	}
	connected := serveRequest(public, requestWithBody(t, http.MethodPost, "/api/v1/quick-connect/token", "secret="+secret))
	var session struct{ Token string }
	if connected.Code != http.StatusCreated || json.Unmarshal(connected.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatalf("Quick Connect token = %d %q", connected.Code, connected.Body.String())
	}
	if response := apiCall(t, public, session.Token, http.MethodGet, "/api/v1/library", nil); response.Code != http.StatusOK {
		t.Fatalf("strong Quick Connect session = %d %q", response.Code, response.Body.String())
	}
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/account"}, {http.MethodPost, "/api/v1/me/mfa/setup"}, {http.MethodGet, "/api/v1/openapi.json"}, {http.MethodGet, "/oauth/authorize"}, {http.MethodGet, "/dlna/token/device.xml"}} {
		if response := apiCall(t, public, session.Token, route.method, route.path, nil); response.Code != http.StatusNotFound {
			t.Fatalf("public administrative route %s = %d %q", route.path, response.Code, response.Body.String())
		}
	}
	profileID := storedProfileID(t, dataDir, "Viewer")
	disable := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles/permissions", "id="+profileID+"&rating=all&libraries=all", owner)
	if response := serveRequest(handler, disable); response.Code != http.StatusSeeOther {
		t.Fatalf("disable remote Viewer = %d %q", response.Code, response.Body.String())
	}
	if response := apiCall(t, public, session.Token, http.MethodGet, "/api/v1/library", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked public session = %d %q", response.Code, response.Body.String())
	}
}

func TestPublicTripwireQuarantinesScannerWithoutStoppingLocalAccess(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	public := server.Remote(handler)
	for index, path := range []string{"/.env", "/.git/config", "/wp-login.php"} {
		request := requestWithBody(t, http.MethodGet, path, "")
		request.RemoteAddr = "203.0.113.8:4321"
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", index+1))
		response := serveRequest(public, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("tripwire %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	blockedRequest := requestWithBody(t, http.MethodGet, "/login", "")
	blockedRequest.RemoteAddr = "203.0.113.8:4321"
	blockedRequest.Header.Set("X-Forwarded-For", "198.51.100.200")
	blocked := serveRequest(public, blockedRequest)
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
		t.Fatalf("quarantined scanner = %d %q", blocked.Code, blocked.Body.String())
	}
	local := serveRequest(handler, requestWithCookieRequest(t, http.MethodGet, "/", "", owner))
	if local.Code != http.StatusOK {
		t.Fatalf("local access after tripwire = %d %q", local.Code, local.Body.String())
	}
	settings := serveRequest(handler, requestWithCookieRequest(t, http.MethodGet, "/settings", "", owner))
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), "Public scanner quarantined") {
		t.Fatalf("Owner tripwire warning = %d %q", settings.Code, settings.Body.String())
	}
}

func TestOwnerKillSwitchFailsPublicAccessClosedAndPersists(t *testing.T) { //nolint:cyclop // One scenario proves immediate, local-safe, persistent kill behavior.
	t.Parallel()
	dataDir := t.TempDir()
	config := remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("k", 32), Listen: "127.0.0.1:8443", DataDir: dataDir}
	manager, err := remoteaccess.New(config, remoteaccess.Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, InternetAccess: manager})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	killed := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/remote-access/kill", "", owner))
	if killed.Code != http.StatusNoContent || manager.Status().State != "killed" {
		t.Fatalf("kill switch = %d %q status=%#v", killed.Code, killed.Body.String(), manager.Status())
	}
	if response := serveRequest(handler, requestWithCookieRequest(t, http.MethodGet, "/", "", owner)); response.Code != http.StatusOK {
		t.Fatalf("local access after kill = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner), http.StatusOK, "Allow public access now")
	reopened, err := remoteaccess.New(config, remoteaccess.Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }})
	if err != nil || reopened.Status().State != "killed" {
		t.Fatalf("persisted kill switch = %#v, %v", reopened, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "remote-access.disabled")); err != nil {
		t.Fatal(err)
	}
	reset := serveRequest(handler, requestWithCookieRequest(t, http.MethodDelete, "/api/v1/remote-access/kill", "", owner))
	if reset.Code != http.StatusNoContent || manager.Status().State != "starting" {
		t.Fatalf("reset kill switch = %d %q status=%#v", reset.Code, reset.Body.String(), manager.Status())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "remote-access.disabled")); !os.IsNotExist(err) {
		t.Fatalf("kill marker remains after reset: %v", err)
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
