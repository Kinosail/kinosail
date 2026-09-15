package servertest

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/quickconnect"
	"github.com/go-webauthn/webauthn/webauthn"
)

// RemoteAuthorizationFuzzFixture uses one real app broker and its profile-backed application.
type RemoteAuthorizationFuzzFixture struct {
	Broker      *quickconnect.Broker
	Application *quickconnect.Application
}

// RemoteAuthorizationFuzzFactory constructs isolated real-app state for each fuzz input.
type RemoteAuthorizationFuzzFactory func(identitycore.Profile) RemoteAuthorizationFuzzFixture

func (suite SecurityFuzz) RemoteAuthorizationStateMachine(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4})
	f.Add([]byte{2, 4, 3, 2, 4})
	f.Fuzz(func(t *testing.T, actions []byte) {
		if len(actions) > 64 {
			t.Skip()
		}
		profile, err := identitycore.NewProfile("Viewer", "viewer-password", false)
		if err != nil {
			t.Fatal(err)
		}
		profile.Remote = true
		profile.Passkeys = []webauthn.Credential{{ID: []byte("passkey")}}
		state := remoteAuthorizationFuzzState{fixture: suite.NewAuthorization(profile), profile: profile}
		state.reset(t)
		for _, action := range actions {
			state.apply(t, action)
		}
	})
}

type remoteAuthorizationFuzzState struct {
	fixture      RemoteAuthorizationFuzzFixture
	profile      identitycore.Profile
	secret, code string
	killed       bool
}

func (state *remoteAuthorizationFuzzState) reset(t *testing.T) {
	t.Helper()
	secret, connection, err := state.fixture.Broker.Create(quickconnect.Request{Device: "TV", Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	state.secret, state.code = secret, connection.Code
}

func (state *remoteAuthorizationFuzzState) apply(t *testing.T, action byte) {
	t.Helper()
	switch action % 5 {
	case 0:
		if err := state.fixture.Application.Approve(identitycore.Profile{ID: "owner", Owner: true}, state.code, true); err == nil {
			t.Fatal("Owner approved a public Quick Connect request")
		}
	case 1:
		if err := state.fixture.Application.Approve(state.profile, state.code, false); err == nil {
			t.Fatal("weak session approved a public Quick Connect request")
		}
	case 2:
		_ = state.fixture.Application.Approve(state.profile, state.code, true)
	case 3:
		state.fixture.Application.RevokeRemote()
		state.killed = true
	case 4:
		_, _, consumeErr := state.fixture.Application.Consume(state.secret)
		if state.killed && consumeErr == nil {
			t.Fatal("kill allowed a pending remote grant to mint a session")
		}
		state.reset(t)
		state.killed = false
	}
}
