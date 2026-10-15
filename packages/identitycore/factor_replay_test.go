package identitycore

import (
	"encoding/base32"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFactorVerifyRejectsReplayedTOTPCounters(t *testing.T) {
	t.Parallel()
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // Public RFC 6238 test key.
	fixture := newFactorFixture(FactorProfile{ID: "owner", Secret: secret, Revision: 2})
	at := func(seconds int64) func() time.Time { return func() time.Time { return time.Unix(seconds, 0) } }
	code := totp(mustTOTPKey(t, secret), 1)
	if !fixture.transaction(at(59)).Verify("owner", code) || fixture.profiles[0].factor.Counter != 1 || fixture.writes != 1 {
		t.Fatalf("first use = counter %d writes %d", fixture.profiles[0].factor.Counter, fixture.writes)
	}
	if fixture.transaction(at(59)).Verify("owner", code) || fixture.transaction(at(75)).Verify("owner", code) || fixture.writes != 1 {
		t.Fatal("replayed code was accepted")
	}
	if fixture.transaction(at(59)).Verify("owner", totp(mustTOTPKey(t, secret), 0)) || fixture.writes != 1 {
		t.Fatal("older window code was accepted after a newer counter")
	}
	next := totp(mustTOTPKey(t, secret), 2)
	if !fixture.transaction(at(59)).Verify("owner", next) || fixture.profiles[0].factor.Counter != 2 || fixture.writes != 2 {
		t.Fatalf("next window = counter %d writes %d", fixture.profiles[0].factor.Counter, fixture.writes)
	}
	if fixture.profiles[0].factor.Revision != 2 || fixture.related || !fixture.atomic || !reflect.DeepEqual(fixture.persistedProfiles, fixture.profiles) {
		t.Fatalf("counter commit = %#v related %t atomic %t", fixture.profiles[0].factor, fixture.related, fixture.atomic)
	}
	if err := fixture.transaction(at(90)).Enable("owner", secret, []string{"code"}); err != nil || fixture.profiles[0].factor.Counter != 0 {
		t.Fatalf("enable reset = %v counter %d", err, fixture.profiles[0].factor.Counter)
	}
	if err := fixture.transaction(at(90)).Disable("owner"); err != nil || fixture.profiles[0].factor.Counter != 0 {
		t.Fatalf("disable reset = %v counter %d", err, fixture.profiles[0].factor.Counter)
	}
}

func TestFactorVerifyRejectsTOTPWhenCounterPersistenceFails(t *testing.T) {
	t.Parallel()
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // Public RFC 6238 test key.
	fixture := newFactorFixture(FactorProfile{ID: "owner", Secret: secret})
	fixture.fail = errors.New("persist failed")
	before := cloneFactorRecords(fixture.profiles)
	if fixture.transaction(func() time.Time { return time.Unix(59, 0) }).Verify("owner", "287082") || fixture.writes != 1 || !reflect.DeepEqual(fixture.profiles, before) {
		t.Fatal("unpersisted counter accepted the code")
	}
}

func mustTOTPKey(t *testing.T, secret string) []byte {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return key
}
