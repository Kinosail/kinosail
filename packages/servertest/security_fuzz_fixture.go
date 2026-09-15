package servertest

import (
	"net"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// NewSecurityFuzz creates isolated real app state with Player's canonical fuzz policy.
func NewSecurityFuzz[Profiles, Broker any](allowed func(net.IP) bool, unsafe func(*http.Request) bool, initialize func(identitycore.Profile, func(string, any) error) Profiles, newBroker func(time.Duration) Broker, bind func(Broker, Profiles) RemoteAuthorizationFuzzFixture) SecurityFuzz {
	return SecurityFuzz{
		AllowedOutboundIP: allowed,
		UnsafeCrossOrigin: unsafe,
		NewAuthorization: func(profile identitycore.Profile) RemoteAuthorizationFuzzFixture {
			profiles := initialize(profile, func(string, any) error { return nil })
			broker := newBroker(time.Minute)
			return bind(broker, profiles)
		},
	}
}
