package mcpgateway

import (
	"net/http"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// ProfileState binds Player's canonical profile state to application adapters.
type ProfileState struct {
	Mutex    *sync.RWMutex
	Profiles *[]identitycore.Profile
	Error    *error
}

// ProfilePrincipalConfig contains the application-specific HTTP identity operations.
type ProfilePrincipalConfig struct {
	State                 ProfileState
	CurrentProfile        func(*http.Request) identitycore.Profile
	RecentlyAuthenticated func(*http.Request, time.Duration) bool
	AttributeProfile      func(*http.Request, identitycore.Profile)
	PublicRequest         func(*http.Request) bool
}

// NewProfilePrincipals binds canonical Player profiles to MCP authorization.
func NewProfilePrincipals(config ProfilePrincipalConfig) PrincipalRepository {
	if config.State.Mutex == nil || config.State.Profiles == nil || config.State.Error == nil || config.CurrentProfile == nil || config.RecentlyAuthenticated == nil || config.AttributeProfile == nil || config.PublicRequest == nil {
		return nil
	}
	profiles := identitycore.NewProfileStore(identitycore.ProfileStoreConfig{
		Mutex: config.State.Mutex, Profiles: config.State.Profiles,
	})
	return NewPrincipalAdapter(PrincipalAdapterConfig[identitycore.Profile]{
		CurrentProfile: config.CurrentProfile, FindProfile: profiles.ByID, FederatedProfiles: profiles.FederatedProfiles,
		AllowProfile: func(profile identitycore.Profile, request *http.Request, now time.Time) bool {
			return profile.Allowed(config.PublicRequest(request), now)
		},
		RecentAuthentication: config.RecentlyAuthenticated, Profiles: profiles.List,
		StateError: func() error {
			config.State.Mutex.RLock()
			defer config.State.Mutex.RUnlock()
			return *config.State.Error
		},
		AttributeProfile: config.AttributeProfile, ConvertProfile: profilePrincipal,
	})
}

func profilePrincipal(profile identitycore.Profile) Principal {
	return Principal{ID: profile.ID, Name: profile.Name, Owner: profile.Owner}
}
