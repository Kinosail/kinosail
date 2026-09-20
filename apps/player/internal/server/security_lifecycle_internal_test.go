package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/federation"
)

func TestFederationLinkRejectsRevokedOriginalAppSession(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := addSecurityTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	original, err := store.createStrongSession(profile.ID, "Browser", true)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := store.createStrongSession(profile.ID, "Browser", true)
	if err != nil || replacement == original {
		t.Fatal("replacement session missing")
	}
	delete(store.sessions, sessionKey(original))
	identity := federation.Identity{Issuer: "https://identity.example", Subject: "owner"}
	if err := store.federatedProfiles().LinkForSession(federation.OIDCProtocol, profile.ID, identity, sessionKey(original)); err == nil {
		t.Fatal("revoked original session linked identity")
	}
	if _, found := store.federatedProfiles().Find(federation.OIDCProtocol, identity); found {
		t.Fatal("rejected link persisted")
	}
	if err := store.federatedProfiles().LinkForSession(federation.OIDCProtocol, profile.ID, identity, sessionKey(replacement)); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordResetInvalidatesPreviouslyApprovedLocalGrant(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := addSecurityTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	revision := store.profiles[0].Revision
	if err := store.resetPassword(profile.ID, "replacement-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.sessionModule().CreateLocalGrant(profile.ID, "TV", revision, false); err == nil {
		t.Fatal("old Quick Connect approval created a session")
	}
	if len(store.sessions) != 0 {
		t.Fatal("rejected approval persisted a session")
	}
}
func TestFederatedSignInCanEnrollAFirstFactor(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := addSecurityTestOwner(store, profile); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/oidc/callback", nil)
	if err := federationWebHooks(store).SignIn(response, request, profile.ID); err != nil {
		t.Fatal(err)
	}
	enrollment := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/me/mfa/setup", nil)
	for _, cookie := range response.Result().Cookies() {
		enrollment.AddCookie(cookie)
	}
	if !store.recentlyAuthenticated(enrollment, 10*time.Minute) {
		t.Fatal("fresh SSO could not enroll first factor")
	}
}

func addSecurityTestOwner(store *profileStore, profile viewerProfile) error {
	profiles := []viewerProfile{profile}
	if err := store.save(profiles); err != nil {
		return err
	}
	store.profiles = profiles
	return nil
}
