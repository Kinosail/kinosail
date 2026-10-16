package identitycore

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFailedSessionWritesLeaveEveryKeyUnchanged(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*Sessions) error{
		"create":        func(sessions *Sessions) error { _, err := sessions.Create("viewer", "TV", true, false, ""); return err },
		"mark-strong":   func(sessions *Sessions) error { return sessions.MarkStrong("current") },
		"sign-out":      func(sessions *Sessions) error { return sessions.SignOut("current") },
		"revoke-others": func(sessions *Sessions) error { return sessions.RevokeOthers("current") },
		"revoke-all":    func(sessions *Sessions) error { return sessions.RevokeAll() },
	} {
		fixture := newSessionFixture()
		expires := fixture.now.Add(time.Hour).Unix()
		fixture.values = map[string]Session{SessionKey("current"): {ProfileID: "viewer", ExpiresAt: expires}, SessionKey("other"): {ProfileID: "viewer", ExpiresAt: expires, StrongAt: 1}}
		before := CloneSessions(fixture.values)
		fixture.persist = errors.New("persist failed")
		if err := mutate(fixture.sessions()); err == nil || fixture.writes != 1 || !reflect.DeepEqual(fixture.values, before) {
			t.Fatalf("%s failure changed state = %#v writes=%d err=%v", name, fixture.values, fixture.writes, err)
		}
		fixture.persist = nil
		if err := mutate(fixture.sessions()); err != nil || reflect.DeepEqual(fixture.values, before) || !reflect.DeepEqual(fixture.written, fixture.values) {
			t.Fatalf("%s success = %#v written=%#v err=%v", name, fixture.values, fixture.written, err)
		}
	}
}
