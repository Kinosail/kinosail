package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestPasskeyStoreLifecycleRejectsAmbiguousCredentialChanges(t *testing.T) { //nolint:cyclop // One store lifecycle proves every credential lookup and update boundary.
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, _ := newProfile("Viewer", "viewer-password", false)
	if err := addTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	credential := &webauthn.Credential{ID: []byte("credential"), PublicKey: []byte("key")}
	if err := store.addPasskey(profile.ID, nil); err == nil {
		t.Fatal("missing credential accepted")
	}
	if err := store.addPasskey("missing-profile", credential); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if err := store.addPasskey(profile.ID, credential); err != nil {
		t.Fatal(err)
	}
	if err := store.addPasskey(profile.ID, credential); err == nil {
		t.Fatal("duplicate credential accepted")
	}
	discovered, err := store.discoverPasskey(credential.ID, []byte(profile.ID))
	if err != nil || discovered.WebAuthnName() != profile.Name {
		t.Fatalf("discover = %#v, %v", discovered, err)
	}
	if _, err = store.discoverPasskey([]byte("other"), []byte(profile.ID)); err == nil {
		t.Fatal("unknown credential discovered")
	}
	if _, err = store.discoverPasskey(credential.ID, []byte("other")); err == nil {
		t.Fatal("wrong user handle discovered")
	}
	if err = store.updatePasskey(profile.ID, nil); err == nil {
		t.Fatal("missing update accepted")
	}
	if err = store.updatePasskey(profile.ID, &webauthn.Credential{ID: []byte("other")}); err == nil {
		t.Fatal("unknown update accepted")
	}
	credential.Authenticator.SignCount = 9
	if err = store.updatePasskey(profile.ID, credential); err != nil {
		t.Fatal(err)
	}
	if got := store.passkeyInventory(profile.ID); len(got) != 1 || got[0].SignCount != 9 || !got[0].UsageTracked || got[0].LastUsed == 0 || got[0].LastUsedLabel == "" || !got[0].MostRecent {
		t.Fatalf("updated inventory = %#v", got)
	}
	cloned := cloneProfiles(store.profiles)
	cloned[0].Passkeys[0].ID[0] = 'X'
	if store.profiles[0].Passkeys[0].ID[0] == 'X' {
		t.Fatal("passkey clone aliases stored credential bytes")
	}
}

func TestPasskeyManagementResponsesAreRedactedAndStable(t *testing.T) { //nolint:cyclop // The response contract and error mapping are one management boundary.
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
	request := httptest.NewRequestWithContext(withViewer(t.Context(), profile), http.MethodGet, "/api/v1/passkeys", nil)
	response := httptest.NewRecorder()
	auth.listAPI(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"configured":true`) || !strings.Contains(response.Body.String(), `"signCount":0`) || !strings.Contains(response.Body.String(), `"usageTracked":true`) || strings.Contains(response.Body.String(), `"id":"passkey"`) {
		t.Fatalf("list = %d %q", response.Code, response.Body.String())
	}
	for err, status := range map[error]int{errInvalidPasskeyID: http.StatusBadRequest, errPasskeyNotFound: http.StatusNotFound, errLastOwnerFactor: http.StatusConflict, errors.New("storage"): http.StatusInternalServerError} {
		if got := passkeyRemovalStatus(err); got != status {
			t.Fatalf("passkeyRemovalStatus(%v) = %d, want %d", err, got, status)
		}
	}
}
