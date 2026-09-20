package quickconnect

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestLocalApprovalCannotOutliveProfileRevision(t *testing.T) {
	for _, compatibility := range []bool{false, true} {
		fixture := newApplicationFixture(t)
		secret := fixture.approved(t, Request{}, fixture.profiles["viewer"])
		profile := fixture.profiles["viewer"]
		profile.Revision++
		fixture.profiles["viewer"] = profile
		if token, _, err := fixture.application.consumeSession(secret, compatibility); err == nil || token != "" || fixture.writes != 0 || len(fixture.sessions) != 0 {
			t.Fatalf("stale local approval issued session: %q %v", token, err)
		}
	}
}

func TestCompatibilityApprovalCannotIssueOwnerSession(t *testing.T) {
	fixture := newApplicationFixture(t)
	owner := identitycore.Profile{ID: "viewer", Owner: true, Revision: 7}
	fixture.profiles["viewer"] = owner
	projected := fixture.application.adapter.Profiles.Compatibility(owner)
	secret := fixture.approved(t, Request{}, projected)
	token, profile, err := fixture.application.Consume(secret)
	if err != nil || profile.Owner || fixture.sessions[identitycore.SessionKey(token)].Channel != "compatibility" {
		t.Fatalf("compatibility approval upgraded authority: %#v %v", profile, err)
	}
}

func TestApprovalRevisionCheckedAgainAtIssuance(t *testing.T) {
	fixture := newApplicationFixture(t)
	secret := fixture.approved(t, Request{}, fixture.profiles["viewer"])
	find := fixture.application.adapter.Profiles.Find
	fixture.application.adapter.Profiles.Find = func(id string) (identitycore.Profile, bool) {
		profile, found := find(id)
		changed := profile
		changed.Revision++
		fixture.profiles[id] = changed
		return profile, found
	}
	if token, _, err := fixture.application.Consume(secret); err == nil || token != "" || fixture.writes != 0 {
		t.Fatalf("approval race issued a session: %q %v", token, err)
	}
}
