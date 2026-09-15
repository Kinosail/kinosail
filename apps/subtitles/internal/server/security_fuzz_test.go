package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var securityFuzz = servertest.NewSecurityFuzz(allowedOutboundIP, unsafeCrossOrigin,
	func(profile viewerProfile, persist func(string, any) error) *profileStore {
		profiles := newProfileStore("")
		profiles.persist = persist
		profiles.profiles = []viewerProfile{profile}
		return profiles
	}, newQuickConnect, func(broker *quickConnectBroker, profiles *profileStore) servertest.RemoteAuthorizationFuzzFixture {
		return servertest.RemoteAuthorizationFuzzFixture{Broker: broker.connections, Application: broker.application(profiles)}
	})

func FuzzRemoteBoundaryParsers(f *testing.F) {
	securityFuzz.RemoteBoundaryParsers(f)
}

func FuzzExactBrowserOrigin(f *testing.F) {
	securityFuzz.ExactBrowserOrigin(f)
}

func FuzzPersistedMediaShareState(f *testing.F) {
	securityFuzz.PersistedMediaShareState(f)
}

func FuzzRemoteAuthorizationStateMachine(f *testing.F) {
	securityFuzz.RemoteAuthorizationStateMachine(f)
}
