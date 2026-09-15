package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestPasskeyInventoryRedactsCredentialMaterialAndRemovalIsFailClosed(t *testing.T) { //nolint:cyclop // One boundary test checks redaction and every malformed identifier.
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	profile.TOTPSecret = "alternate-factor"
	if err = addTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	credential := webauthn.Credential{ID: []byte("credential-id"), PublicKey: []byte("secret-public-key"), Flags: webauthn.CredentialFlags{BackupEligible: true, BackupState: true}, Authenticator: webauthn.Authenticator{SignCount: 7, CloneWarning: true}}
	if err = store.addPasskey(profile.ID, &credential); err != nil {
		t.Fatal(err)
	}
	items := store.passkeyInventory(profile.ID)
	if len(items) != 1 || len(items[0].ID) != 64 || !items[0].BackupEligible || !items[0].BackedUp || !items[0].CloneWarning || items[0].SignCount != 7 {
		t.Fatalf("inventory = %#v", items)
	}
	encoded := items[0].ID + items[0].Display
	if strings.Contains(encoded, "credential-id") || strings.Contains(encoded, "secret-public-key") {
		t.Fatalf("inventory leaked credential material: %#v", items)
	}
	for _, invalid := range []string{"", strings.ToUpper(items[0].ID), items[0].ID + "0", strings.Repeat("g", 64)} {
		if err = store.removePasskey(profile.ID, invalid); err == nil || len(store.passkeyInventory(profile.ID)) != 1 {
			t.Fatalf("invalid id %q changed inventory: %v", invalid, err)
		}
	}
	if err = store.removePasskey(profile.ID, strings.Repeat("0", 64)); err == nil || len(store.passkeyInventory(profile.ID)) != 1 {
		t.Fatalf("unknown id changed inventory: %v", err)
	}
	if err = store.removePasskey(profile.ID, items[0].ID); err != nil || len(store.passkeyInventory(profile.ID)) != 0 {
		t.Fatalf("valid removal = %v, inventory = %#v", err, store.passkeyInventory(profile.ID))
	}
}

func TestOwnerCannotRemoveOnlyStrongFactor(t *testing.T) {
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, _ := newProfile("Owner", "owner-password", true)
	if err := addTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	if err := store.addPasskey(profile.ID, &webauthn.Credential{ID: []byte("only-passkey"), PublicKey: []byte("key")}); err != nil {
		t.Fatal(err)
	}
	item := store.passkeyInventory(profile.ID)[0]
	if err := store.removePasskey(profile.ID, item.ID); err == nil || len(store.passkeyInventory(profile.ID)) != 1 {
		t.Fatalf("only-factor removal = %v", err)
	}
}

func TestPasskeyRemovalRequiresRecentStrongAuthentication(t *testing.T) { //nolint:cyclop,funlen // One security lifecycle proves stale, weak, invalid, and valid removal attempts.
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, _ := newProfile("Viewer", "viewer-password", false)
	if err := addTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	if err := store.addPasskey(profile.ID, &webauthn.Credential{ID: []byte("passkey"), PublicKey: []byte("key")}); err != nil {
		t.Fatal(err)
	}
	auth := &passkeyAuth{profiles: store}
	id := store.passkeyInventory(profile.ID)[0].ID

	request := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/passkeys/"+id, nil)
	request.SetPathValue("id", id)
	request = request.WithContext(withViewer(request.Context(), profile))
	denied := httptest.NewRecorder()
	auth.removeAPI(denied, request)
	if denied.Code != http.StatusForbidden || len(store.passkeyInventory(profile.ID)) != 1 {
		t.Fatalf("unstaged removal = %d %q", denied.Code, denied.Body.String())
	}

	token, err := store.createStrongSession(profile.ID, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	invalidForm := url.Values{"id": {strings.ToUpper(id)}}
	webRequest := httptest.NewRequestWithContext(withViewer(t.Context(), profile), http.MethodPost, "/account/passkeys/remove", strings.NewReader(invalidForm.Encode()))
	webRequest.Header.Set("Authorization", "Bearer "+token)
	webRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidWeb := httptest.NewRecorder()
	auth.removeWeb(invalidWeb, webRequest)
	if invalidWeb.Code != http.StatusBadRequest || len(store.passkeyInventory(profile.ID)) != 1 {
		t.Fatalf("invalid web removal = %d %q", invalidWeb.Code, invalidWeb.Body.String())
	}
	removed := httptest.NewRecorder()
	auth.removeAPI(removed, request)
	if removed.Code != http.StatusNoContent || len(store.passkeyInventory(profile.ID)) != 0 {
		t.Fatalf("strong removal = %d %q", removed.Code, removed.Body.String())
	}
	if err = store.addPasskey(profile.ID, &webauthn.Credential{ID: []byte("web-passkey"), PublicKey: []byte("key")}); err != nil {
		t.Fatal(err)
	}
	webID := store.passkeyInventory(profile.ID)[0].ID
	form := url.Values{"id": {webID}}
	webRequest = httptest.NewRequestWithContext(withViewer(t.Context(), profile), http.MethodPost, "/account/passkeys/remove", strings.NewReader(form.Encode()))
	webRequest.Header.Set("Authorization", "Bearer "+token)
	webRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	web := httptest.NewRecorder()
	auth.removeWeb(web, webRequest)
	if web.Code != http.StatusSeeOther || web.Header().Get("Location") != "/account" || len(store.passkeyInventory(profile.ID)) != 0 {
		t.Fatalf("web removal = %d %q", web.Code, web.Body.String())
	}
}

func TestPasskeyCloneWarningCreatesSecuritySignalWithoutAutomaticRevocation(t *testing.T) {
	audit := newAuditStore(t.Context(), "", nil)
	servertest.PasskeyCloneWarningCreatesSecuritySignal(t, audit.passkeyRisk, audit.Query)
}
