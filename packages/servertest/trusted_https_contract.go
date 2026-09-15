package servertest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TrustedHTTPSFixture binds each app's real configuration and HTTP handlers.
type TrustedHTTPSFixture struct {
	New                   func(*testing.T, string) http.Handler
	Load                  func(string) (raw, listen string, err error)
	Origin                func(string, string) (string, error)
	SignIn                func(*testing.T, http.Handler, string, string) *http.Cookie
	Web                   AuthCookieRequest
	Token, ExpectedOrigin string
}

type trustedHTTPSFlow struct {
	t         *testing.T
	fixture   TrustedHTTPSFixture
	handler   http.Handler
	owner     *http.Cookie
	directory string
}

// AssertOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb preserves persistence, redaction and disablement parity.
func AssertOwnerCanConfigureTrustedHTTPSThroughAPIAndWeb(t *testing.T, fixture TrustedHTTPSFixture) {
	t.Parallel()
	directory := t.TempDir()
	handler := fixture.New(t, directory)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	flow := trustedHTTPSFlow{t: t, fixture: fixture, handler: handler, owner: owner, directory: directory}
	flow.initial()
	flow.save()
	flow.render()
	flow.update()
	flow.disable()
}

func (flow trustedHTTPSFlow) initial() {
	t, fixture, handler, owner := flow.t, flow.fixture, flow.handler, flow.owner
	t.Helper()
	initial := fixture.Web(t, handler, http.MethodGet, "/settings", "", owner)
	for _, expected := range []string{"Trusted HTTPS (Required for Jellyfin apps)", "Required for Jellyfin apps. Recommended for phones and TVs.", `action="/settings/trusted-https"`, "does not open a router port", "certificate-transparency logs"} {
		if initial.Code != http.StatusOK || !strings.Contains(initial.Body.String(), expected) {
			t.Fatalf("initial settings lacks %q: %d %q", expected, initial.Code, initial.Body.String())
		}
	}
}

func (flow trustedHTTPSFlow) save() {
	t, fixture, handler, owner, directory := flow.t, flow.fixture, flow.handler, flow.owner, flow.directory
	t.Helper()
	saved := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "Family-Media", "token": fixture.Token, "address": "192.168.1.10", "termsAccepted": true})
	if saved.Code != http.StatusAccepted || !strings.Contains(saved.Body.String(), `"hostname":"family-media.duckdns.org"`) || !strings.Contains(saved.Body.String(), `"restartRequired":true`) || strings.Contains(saved.Body.String(), fixture.Token) {
		t.Fatalf("save = %d %q", saved.Code, saved.Body.String())
	}
	regular, _ := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secrets, secretErr := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if secretErr != nil || strings.Contains(string(regular), fixture.Token) || !strings.Contains(string(secrets), fixture.Token) {
		t.Fatalf("regular=%q secrets=%q err=%v", regular, secrets, secretErr)
	}
}

func (flow trustedHTTPSFlow) render() {
	t, fixture, handler, owner := flow.t, flow.fixture, flow.handler, flow.owner
	t.Helper()
	settings := APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil)
	page := fixture.Web(t, handler, http.MethodGet, "/settings", "", owner)
	if settings.Code != http.StatusOK || page.Code != http.StatusOK || strings.Contains(settings.Body.String(), fixture.Token) || strings.Contains(page.Body.String(), fixture.Token) || !strings.Contains(settings.Body.String(), `"tokenConfigured":true`) || !strings.Contains(page.Body.String(), `value="family-media"`) || !strings.Contains(page.Body.String(), `value="192.168.1.10"`) || !strings.Contains(page.Body.String(), "Leave blank to keep the current token") {
		t.Fatalf("settings=%d %q page=%d %q", settings.Code, settings.Body.String(), page.Code, page.Body.String())
	}
}

func (flow trustedHTTPSFlow) update() {
	t, fixture, handler, owner, directory := flow.t, flow.fixture, flow.handler, flow.owner, flow.directory
	t.Helper()
	updated := fixture.Web(t, handler, http.MethodPost, "/settings/trusted-https", "domain=family-media&token=&address=192.168.1.11&termsAccepted=true", owner)
	secrets, secretErr := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if updated.Code != http.StatusSeeOther || secretErr != nil || !strings.Contains(string(secrets), fixture.Token) || !strings.Contains(string(secrets), "192.168.1.11") {
		t.Fatalf("web update=%d secrets=%q err=%v", updated.Code, secrets, secretErr)
	}
	raw, listen, loadErr := fixture.Load(directory)
	origin, originErr := fixture.Origin(raw, listen)
	if loadErr != nil || originErr != nil || origin != fixture.ExpectedOrigin {
		t.Fatalf("reloaded origin = %q, load=%v origin=%v", origin, loadErr, originErr)
	}
}

func (flow trustedHTTPSFlow) disable() {
	t, fixture, handler, owner, directory := flow.t, flow.fixture, flow.handler, flow.owner, flow.directory
	t.Helper()
	disabled := fixture.Web(t, handler, http.MethodPost, "/settings/trusted-https/disable", "", owner)
	raw, _, loadErr := fixture.Load(directory)
	if disabled.Code != http.StatusSeeOther || disabled.Header().Get("Location") != "/settings#trusted-https" || loadErr != nil || raw != "" {
		t.Fatalf("disable=%d location=%q raw=%q err=%v", disabled.Code, disabled.Header().Get("Location"), raw, loadErr)
	}
	resaved := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/trusted-https", map[string]any{"domain": "family-media", "token": fixture.Token, "address": "192.168.1.10", "termsAccepted": true})
	apiDisabled := APICall(t, handler, owner.Value, http.MethodDelete, "/api/v1/settings/trusted-https", nil)
	if resaved.Code != http.StatusAccepted || apiDisabled.Code != http.StatusAccepted || !strings.Contains(apiDisabled.Body.String(), `"restartRequired":true`) {
		t.Fatalf("API resave=%d %q disable=%d %q", resaved.Code, resaved.Body.String(), apiDisabled.Code, apiDisabled.Body.String())
	}
}
