package auditjournal

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func auditRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), method, path, nil)
}

func TestClassifyUsesPlayerDefaultsAndApplicationOverrides(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		method, path, action, category string
		overrides                      map[string]string
	}{
		"read":                   {http.MethodGet, "/library", "", "", nil},
		"playback":               {http.MethodPost, "/progress/id", "", "", nil},
		"session create":         {http.MethodPost, "/api/v1/session", "session.created", "security", nil},
		"session delete":         {http.MethodDelete, "/api/v1/session", "session.ended", "security", nil},
		"Jellyfin password":      {http.MethodPost, "/Users/AuthenticateByName", "session.created", "security", nil},
		"Jellyfin quick":         {http.MethodPost, "/Users/AuthenticateWithQuickConnect", "session.created", "security", nil},
		"Quick Connect":          {http.MethodPost, "/QuickConnect/Authorize", "quick-connect.approved", "security", nil},
		"static":                 {http.MethodPost, "/setup", "owner.created", "administration", nil},
		"static delete choice":   {http.MethodDelete, "/api/v1/libraries", "library.removed", "administration", nil},
		"static update choice":   {http.MethodPost, "/api/v1/libraries", "library.added", "administration", nil},
		"disable trusted HTTPS":  {http.MethodDelete, "/api/v1/settings/trusted-https", "settings.trusted-https.disabled", "administration", nil},
		"update trusted HTTPS":   {http.MethodPut, "/api/v1/settings/trusted-https", "settings.trusted-https.updated", "administration", nil},
		"override add":           {http.MethodPost, "/custom", "custom.changed", "administration", map[string]string{"/custom": "custom.changed"}},
		"override remove":        {http.MethodPost, "/setup", "activity.post", "activity", map[string]string{"/setup": ""}},
		"configuration update":   {http.MethodPut, "/api/v1/configuration/name", "configuration.updated", "administration", nil},
		"configuration delete":   {http.MethodDelete, "/api/v1/configuration/name", "configuration.reset", "administration", nil},
		"unknown admin":          {http.MethodPatch, "/settings/future", "administration.patch", "administration", nil},
		"unknown metadata admin": {http.MethodPost, "/metadata/future", "metadata.updated", "administration", nil},
		"unknown activity":       {http.MethodPost, "/future", "activity.post", "activity", nil},
		"OIDC callback":          {http.MethodGet, "/login/oidc/callback", "sso.session", "administration", nil},
	} {
		action, category := Classify(auditRequest(t, test.method, test.path), test.overrides)
		if action != test.action || category != test.category {
			t.Fatalf("%s = %q %q, want %q %q", name, action, category, test.action, test.category)
		}
	}
}

func TestDynamicAuditActionsCoverEveryRouteFamily(t *testing.T) { //nolint:cyclop // The table fixes each route family to its stable event name.
	t.Parallel()
	cases := []struct{ method, path, action, category string }{
		{http.MethodPost, "/scim/v2/Users", "scim.profile.provisioned", "administration"},
		{http.MethodPut, "/scim/v2/Users/id", "scim.profile.provisioned", "administration"},
		{http.MethodPatch, "/scim/v2/Users/id", "scim.profile.provisioned", "administration"},
		{http.MethodDelete, "/scim/v2/Users/id", "scim.profile.deprovisioned", "administration"},
		{http.MethodPost, "/api/v1/profiles", "profile.created", "administration"},
		{http.MethodPut, "/api/v1/profiles/id", "profile.permissions", "administration"},
		{http.MethodDelete, "/api/v1/profiles/id", "profile.removed", "administration"},
		{http.MethodPut, "/api/v1/profiles/id/password", "profile.password", "administration"},
		{http.MethodPost, "/api/v1/api-keys", "api-key.created", "administration"},
		{http.MethodDelete, "/api/v1/api-keys/id", "api-key.revoked", "administration"},
		{http.MethodDelete, "/api/v1/devices/id", "session.revoked", "administration"},
		{http.MethodDelete, "/api/v1/sessions", "sessions.revoked", "administration"},
		{http.MethodPost, "/api/v1/tasks/scan", "task.scan", "administration"},
		{http.MethodPost, "/api/v1/backups", "backup.created", "administration"},
		{http.MethodPost, "/api/v1/backups/id/verify", "backup.verified", "administration"},
		{http.MethodPost, "/api/v1/remote-access/wireguard", "wireguard-peer.created", "administration"},
		{http.MethodDelete, "/api/v1/remote-access/wireguard/id", "wireguard-peer.revoked", "administration"},
		{http.MethodPost, "/api/v1/remote-access/kill", "remote-access.killed", "administration"},
		{http.MethodDelete, "/api/v1/remote-access/kill", "remote-access.reset", "administration"},
		{http.MethodPost, "/api/v1/media-shares", "media-share.created", "activity"},
		{http.MethodDelete, "/api/v1/media-shares/id", "media-share.revoked", "activity"},
		{http.MethodPost, "/api/v1/media-shares/id/claim", "media-share.claimed", "activity"},
		{http.MethodPost, "/settings/media-shares/id/revoke", "media-share.revoked", "administration"},
		{http.MethodPost, "/api/v1/items/id/metadata/refresh", "metadata.refreshed", "administration"},
		{http.MethodPut, "/api/v1/items/id/metadata", "metadata.updated", "administration"},
		{http.MethodPost, "/api/v1/items/id/markers", "marker.updated", "activity"},
		{http.MethodDelete, "/api/v1/items/id/markers", "marker.removed", "activity"},
		{http.MethodPost, "/api/v1/items/id/subtitles", "subtitle.fetched", "administration"},
		{http.MethodPost, "/api/v1/collections", "collection.updated", "administration"},
		{http.MethodDelete, "/collection/name", "collection.removed", "activity"},
		{http.MethodPost, "/collections", "collection.updated", "activity"},
		{http.MethodPut, "/api/v1/playlists/name", "playlist.updated", "activity"},
		{http.MethodDelete, "/playlist/name", "playlist.removed", "activity"},
		{http.MethodPost, "/api/v1/passkeys/register/begin", "passkey.registered", "activity"},
		{http.MethodDelete, "/api/v1/passkeys/id", "passkey.removed", "activity"},
		{http.MethodPut, "/api/v1/me/mfa", "mfa.enabled", "activity"},
		{http.MethodDelete, "/api/v1/me/mfa", "mfa.disabled", "activity"},
		{http.MethodDelete, "/api/v1/me/oidc", "oidc.unlinked", "activity"},
		{http.MethodPost, "/offline/id", "download.created", "activity"},
		{http.MethodDelete, "/api/v1/downloads/id", "download.removed", "activity"},
		{http.MethodPost, "/list/id", "list.updated", "activity"},
		{http.MethodPost, "/api/v1/items/id/list", "list.updated", "activity"},
	}
	for _, test := range cases {
		action, category := Classify(auditRequest(t, test.method, test.path), nil)
		if action != test.action || category != test.category {
			t.Fatalf("%s %s = %q %q, want %q %q", test.method, test.path, action, category, test.action, test.category)
		}
	}
}

func TestPlaybackAndAdministrativeClassifiersCoverAllShapes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/progress/id", "/watched/id", "/api/v1/items/id/progress", "/Users/id/PlayedItems/id", "/Sessions/Playing", "/Sessions/Playing/Progress", "/Sessions/Capabilities", "/Sessions/Capabilities/Full"} {
		if !playbackMutation(path) {
			t.Fatalf("playback path = %q", path)
		}
	}
	if playbackMutation("/library") {
		t.Fatal("library classified as playback mutation")
	}
	for _, path := range []string{"/api/v1/settings/name", "/api/v1/profiles", "/api/v1/devices", "/api/v1/sessions", "/api/v1/api-keys", "/api/v1/tasks", "/api/v1/remote-access", "/api/v1/items/id/metadata", "/api/v1/items/id/subtitles", "/api/v1/collections", "/api/v1/backups", "/api/v1/updates"} {
		if !apiAdministrativeChange(auditRequest(t, http.MethodPost, path)) {
			t.Fatalf("admin path = %q", path)
		}
	}
	if apiAdministrativeChange(auditRequest(t, http.MethodPost, "/api/v1/library")) {
		t.Fatal("library classified as administration")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestAuditTargetAndDetailsRedactUntrustedInput(t *testing.T) { //nolint:cyclop,gocognit // The cases cover every supported input representation.
	t.Parallel()
	for key := range map[string]bool{"key": true, "name": true, "device": true, "path": true, "id": true} {
		request := auditRequest(t, http.MethodPost, "/target")
		request.Form = url.Values{key: {strings.Repeat("x", 121)}}
		if target := Target(request); len(target) != 120 {
			t.Fatalf("%s target length = %d", key, len(target))
		}
	}
	if Target(auditRequest(t, http.MethodPost, "/fallback")) != "/fallback" {
		t.Fatal("target did not fall back to path")
	}
	request := auditRequest(t, http.MethodPost, "/settings/configuration")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Form = url.Values{"key": {"backup.token"}, "name": {"Cabin"}, "title": {"password"}, "owner": {}, "ignored": {"secret"}}
	details := Details(request)
	if details["key"] != "backup.token" || details["value"] != "[redacted]" || details["name"] != "Cabin" || details["title"] != "password" {
		t.Fatalf("form details = %#v", details)
	}
	if _, found := details["owner"]; found {
		t.Fatalf("empty allowed form detail was retained: %#v", details)
	}
	if _, found := details["ignored"]; found {
		t.Fatalf("unknown form detail was retained: %#v", details)
	}
	ordinary := auditRequest(t, http.MethodPost, "/settings/configuration")
	ordinary.Form = url.Values{"title": {"password"}}
	if details := Details(ordinary); details["title"] != "password" || details["value"] != "" {
		t.Fatalf("ordinary secret-looking value = %#v", details)
	}
	jsonRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/json", strings.NewReader(`{"name":"Cabin","owner":true,"year":2026,"libraries":["one",2,false],"unknown":null,"password":"hidden"}`))
	jsonRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	details = Details(jsonRequest)
	if details["name"] != "Cabin" || details["owner"] != "true" || details["year"] != "2026" || details["libraries"] != "one,2,false" || details["unknown"] != "" || details["password"] != "" {
		t.Fatalf("JSON details = %#v", details)
	}
	for _, path := range []string{"/Users/AuthenticateByName", "/Users/AuthenticateWithQuickConnect"} {
		request := auditRequest(t, http.MethodPost, path)
		if details := Details(request); details["channel"] == "" || details["privileges"] != "media-only" {
			t.Fatalf("compatibility details = %#v", details)
		}
	}
	invalid := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/json", strings.NewReader(`{"name":`))
	invalid.Header.Set("Content-Type", "application/json")
	if len(Details(invalid)) != 0 {
		t.Fatal("invalid JSON produced details")
	}
	failed := auditRequest(t, http.MethodPost, "/json")
	failed.Header.Set("Content-Type", "application/json")
	failed.Body = io.NopCloser(failingReader{})
	if len(Details(failed)) != 0 {
		t.Fatal("failed body read produced details")
	}
}

func TestAuditStringBoundsAndSecretNames(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"Password", "client_secret", "access-token", "backup.key"} {
		if !SecretSetting(key) {
			t.Fatalf("secret key = %q", key)
		}
	}
	if SecretSetting("server.name") {
		t.Fatal("ordinary setting classified as secret")
	}
	if Truncate("short", 5) != "short" || Truncate("longer", 4) != "long" {
		t.Fatal("truncate bounds failed")
	}
}
