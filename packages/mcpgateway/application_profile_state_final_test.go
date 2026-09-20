package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

type profilePrincipalContextKey struct{}

func profilePrincipalTestConfig(mutex *sync.RWMutex, profiles *[]identitycore.Profile, stateErr *error) ProfilePrincipalConfig {
	return ProfilePrincipalConfig{
		MFARequired: func() bool { return false },
		State:       ProfileState{Mutex: mutex, Profiles: profiles, Error: stateErr},
		CurrentProfile: func(request *http.Request) identitycore.Profile {
			profile, _ := request.Context().Value(profilePrincipalContextKey{}).(identitycore.Profile)
			return profile
		},
		RecentlyAuthenticated: func(*http.Request, time.Duration) bool { return true },
		AttributeProfile:      func(*http.Request, identitycore.Profile) {},
		PublicRequest:         func(*http.Request) bool { return false },
	}
}

func TestProfilePrincipalsOwnCanonicalIdentityPolicy(t *testing.T) { //nolint:cyclop // One flow covers the shared identity contract.
	var mutex sync.RWMutex
	profiles := []identitycore.Profile{
		{ID: "owner", Name: "Owner", Owner: true, TOTPSecret: "secret", OIDCIssuer: "https://identity.test", OIDCSubject: "subject"},
		{ID: "viewer", Name: "Viewer", Remote: false},
	}
	var stateErr error
	public := false
	attributed := identitycore.Profile{}
	config := profilePrincipalTestConfig(&mutex, &profiles, &stateErr)
	config.PublicRequest = func(*http.Request) bool { return public }
	config.AttributeProfile = func(_ *http.Request, profile identitycore.Profile) { attributed = profile }
	principals := NewProfilePrincipals(config)
	request := httptest.NewRequestWithContext(context.WithValue(t.Context(), profilePrincipalContextKey{}, profiles[0]), http.MethodGet, "/", nil)
	if current := principals.Current(request); current != (Principal{ID: "owner", Name: "Owner", Owner: true}) {
		t.Fatalf("current principal = %+v", current)
	}
	if found, ok := principals.ByID("viewer"); !ok || found != (Principal{ID: "viewer", Name: "Viewer"}) {
		t.Fatalf("profile lookup = %+v, %v", found, ok)
	}
	if found, ok := principals.ByOIDC("https://identity.test", "subject"); !ok || found.ID != "owner" {
		t.Fatalf("OIDC lookup = %+v, %v", found, ok)
	}
	if !principals.Allowed(Principal{ID: "viewer"}, request, time.Now()) {
		t.Fatal("private request was denied")
	}
	public = true
	if principals.Allowed(Principal{ID: "viewer"}, request, time.Now()) || !principals.Allowed(Principal{ID: "owner"}, request, time.Now()) {
		t.Fatal("public access policy changed")
	}
	if !principals.RecentlyAuthenticated(request, time.Minute) {
		t.Fatal("recent authentication callback changed")
	}
	owners, err := principals.Owners()
	if err != nil || !slices.Equal(owners, []Principal{{ID: "owner", Name: "Owner", Owner: true}}) {
		t.Fatalf("owners = %+v, %v", owners, err)
	}
	principals.Attribute(request, Principal{ID: "viewer"})
	if attributed.ID != "viewer" {
		t.Fatalf("attributed profile = %+v", attributed)
	}
	stateErr = errors.New("profile state unavailable")
	if owners, err = principals.Owners(); !errors.Is(err, stateErr) || owners != nil {
		t.Fatalf("unavailable owners = %+v, %v", owners, err)
	}
}

func TestProfilePrincipalsRejectIncompleteStateWithoutCallbacks(t *testing.T) {
	var mutex sync.RWMutex
	profiles := []identitycore.Profile{{ID: "owner", Owner: true}}
	var stateErr error
	mutations := []func(*ProfilePrincipalConfig){
		func(config *ProfilePrincipalConfig) { config.State.Mutex = nil },
		func(config *ProfilePrincipalConfig) { config.State.Profiles = nil },
		func(config *ProfilePrincipalConfig) { config.State.Error = nil },
		func(config *ProfilePrincipalConfig) { config.CurrentProfile = nil },
		func(config *ProfilePrincipalConfig) { config.RecentlyAuthenticated = nil },
		func(config *ProfilePrincipalConfig) { config.AttributeProfile = nil },
		func(config *ProfilePrincipalConfig) { config.PublicRequest = nil },
	}
	for index, mutate := range mutations {
		config := profilePrincipalTestConfig(&mutex, &profiles, &stateErr)
		mutate(&config)
		if principals := NewProfilePrincipals(config); principals != nil {
			t.Fatalf("incomplete configuration %d produced principals", index)
		}
	}
}
