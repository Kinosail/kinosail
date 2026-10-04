package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/dlna"
	"github.com/MikeO7/kinosail/packages/library"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestStrongPublicSessionCookieAndPasskeyInventoryShape(t *testing.T) {
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, _ := newProfile("Viewer", "viewer-password", false)
	profile.Passkeys = []webauthn.Credential{{ID: []byte("credential")}}
	profile.Remote = true
	if err := store.addOwner(profile); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/login", nil)
	response := httptest.NewRecorder()
	if err := store.signInStrongPublic(response, request, profile.ID); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "__Host-kinosail_session" || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("public session cookies = %#v", cookies)
	}
	credentials := sharedpasskeys.CloneAll(profile.Passkeys)
	if len(credentials) != 1 || !strings.EqualFold(string(credentials[0].ID), "credential") {
		t.Fatalf("WebAuthn credentials = %#v", credentials)
	}
}

func TestDLNADiscoveryBoundaries(t *testing.T) {
	t.Parallel()
	response := dlna.SSDPResponse("http://media.test/", "token")
	if !strings.Contains(response, "LOCATION: http://media.test/dlna/token/device.xml") || !strings.Contains(response, "ST: urn:schemas-upnp-org:device:MediaServer:1") || strings.Contains(response, "token?") {
		t.Fatalf("SSDP response = %q", response)
	}
	for kind, want := range map[string]string{"audio": "object.item.audioItem.musicTrack", "photo": "object.item.imageItem.photo", "video": "object.item.videoItem.movie"} {
		if got := dlna.Class(library.Item{Kind: kind}); got != want {
			t.Fatalf("dlnaClass(%q) = %q, want %q", kind, got, want)
		}
	}
}
