package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestManagementIdentityRequiresBoundSecuredRecentlyAuthenticatedOwner(t *testing.T) {
	t.Parallel()
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	profile.TOTPSecret = "secret"
	if err := store.addOwner(profile); err != nil {
		t.Fatal(err)
	}
	token, err := store.createStrongSession(profile.ID, "Browser", true)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), "POST", "/settings", nil)
	request.AddCookie(sessionCookie(token))
	auth := &authentication{profiles: store}
	if !auth.managementIdentityAllowed(profile, request, profile.ID) {
		t.Fatal("authorized management owner rejected")
	}
	for _, kind := range []string{"binding", "viewer", "api key", "unsecured", "stale"} {
		t.Run(kind, func(t *testing.T) {
			candidate, bound := managementIdentityVariant(kind, profile, store, token)
			if auth.managementIdentityAllowed(candidate, request, bound) {
				t.Fatalf("%s identity authorized", kind)
			}
		})
	}
}

func managementIdentityVariant(kind string, profile viewerProfile, store *profileStore, token string) (viewerProfile, string) {
	candidate, bound := profile, profile.ID

	switch kind {
	case "binding":
		bound = "other"
	case "viewer":
		candidate.Owner = false
	case "api key":
		candidate.APIKey = true
	case "unsecured":
		candidate.TOTPSecret = ""
	case "stale":
		state := store.sessions[sessionKey(token)]
		state.StrongAt = time.Now().Add(-9 * time.Hour).Unix()
		store.sessions[sessionKey(token)] = state
	}
	return candidate, bound
}
