package identitycore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type factorRecord struct {
	factor FactorProfile
}

type factorFixture struct {
	profiles          []factorRecord
	sessions          map[string]Session
	fail              error
	writes            int
	related           bool
	atomic            bool
	persistedProfiles []factorRecord
	persistedSessions map[string]Session
}

func newFactorFixture(profile FactorProfile) *factorFixture {
	return &factorFixture{
		profiles: []factorRecord{{factor: profile}},
		sessions: map[string]Session{
			"local":  {ProfileID: profile.ID, Channel: "local"},
			"public": {ProfileID: profile.ID, Channel: "public"},
			"other":  {ProfileID: "other", Channel: "public"},
		},
	}
}

func (fixture *factorFixture) transaction(now func() time.Time) *FactorTransaction[factorRecord] {
	beforeProfiles := cloneFactorRecords(fixture.profiles)
	beforeSessions := CloneSessions(fixture.sessions)
	return NewFactorTransaction(FactorConfig[factorRecord]{
		Profiles: &fixture.profiles,
		Sessions: &fixture.sessions,
		Project:  func(profile factorRecord) FactorProfile { return profile.factor },
		Apply: func(profile factorRecord, factor FactorProfile) factorRecord {
			profile.factor = factor
			return profile
		},
		Persist: func(profiles []factorRecord, sessions map[string]Session, related bool) error {
			fixture.writes++
			fixture.related = related
			fixture.atomic = reflect.DeepEqual(fixture.profiles, beforeProfiles) && reflect.DeepEqual(fixture.sessions, beforeSessions)
			fixture.persistedProfiles = cloneFactorRecords(profiles)
			fixture.persistedSessions = CloneSessions(sessions)
			return fixture.fail
		},
		Now: now,
	})
}

func cloneFactorRecords(profiles []factorRecord) []factorRecord {
	cloned := make([]factorRecord, len(profiles))
	for index, profile := range profiles {
		cloned[index] = profile
		cloned[index].factor.Recovery = append([]string(nil), profile.factor.Recovery...)
	}
	return cloned
}

func TestFactorEnableHashesCodesAndCommitsRelatedState(t *testing.T) { //nolint:cyclop // Exact state assertions protect the complete atomic enrollment transaction.
	t.Parallel()
	fixture := newFactorFixture(FactorProfile{ID: "owner", Owner: true, Revision: 4})
	transaction := fixture.transaction(nil)
	if transaction.config.Now == nil {
		t.Fatal("default clock was not installed")
	}
	if err := transaction.Enable("owner", "secret", []string{" aa-bb ", "CC DD"}); err != nil {
		t.Fatal(err)
	}
	wantRecovery := []string{SessionKey("AABB"), SessionKey("CCDD")}
	got := fixture.profiles[0].factor
	if got.Secret != "secret" || !reflect.DeepEqual(got.Recovery, wantRecovery) || got.Revision != 5 {
		t.Fatalf("enabled factor = %#v", got)
	}
	if fixture.writes != 1 || !fixture.related || !fixture.atomic || !reflect.DeepEqual(fixture.persistedProfiles, fixture.profiles) {
		t.Fatalf("persistence state = writes %d related %t atomic %t", fixture.writes, fixture.related, fixture.atomic)
	}
	if _, found := fixture.sessions["public"]; found || fixture.sessions["local"].ProfileID != "owner" || fixture.sessions["other"].ProfileID != "other" {
		t.Fatalf("sessions after enable = %#v", fixture.sessions)
	}
	if !reflect.DeepEqual(fixture.persistedSessions, fixture.sessions) {
		t.Fatal("persisted sessions do not match the committed sessions")
	}
}

func TestFactorEnableValidatesBeforePersistence(t *testing.T) { //nolint:gocognit // One table proves the complete input boundary has no side effects.
	t.Parallel()
	oversizedRecovery := make([]string, recoveryCodeCount+1)
	for index := range oversizedRecovery {
		oversizedRecovery[index] = string(rune('a' + index))
	}
	tests := []struct {
		name     string
		id       string
		secret   string
		recovery []string
	}{
		{"missing identity", "", "secret", []string{"code"}},
		{"spaced identity", " owner", "secret", []string{"code"}},
		{"oversized identity", strings.Repeat("i", maxIdentityLength+1), "secret", []string{"code"}},
		{"nul identity", "owner\x00", "secret", []string{"code"}},
		{"missing secret", "owner", "", []string{"code"}},
		{"spaced secret", "owner", " secret", []string{"code"}},
		{"oversized secret", "owner", strings.Repeat("s", maxSecretLength+1), []string{"code"}},
		{"nul secret", "owner", "secret\x00", []string{"code"}},
		{"missing recovery", "owner", "secret", nil},
		{"too many recovery codes", "owner", "secret", oversizedRecovery},
		{"empty recovery", "owner", "secret", []string{" -- "}},
		{"oversized recovery", "owner", "secret", []string{strings.Repeat("r", maxSecretLength+1)}},
		{"duplicate recovery", "owner", "secret", []string{"aa-bb", "AABB"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFactorFixture(FactorProfile{ID: "owner", Revision: 2})
			beforeProfiles, beforeSessions := cloneFactorRecords(fixture.profiles), CloneSessions(fixture.sessions)
			if err := fixture.transaction(time.Now).Enable(test.id, test.secret, test.recovery); !errors.Is(err, ErrInvalidFactorInput) {
				t.Fatalf("error = %v", err)
			}
			if fixture.writes != 0 || !reflect.DeepEqual(fixture.profiles, beforeProfiles) || !reflect.DeepEqual(fixture.sessions, beforeSessions) {
				t.Fatal("invalid enrollment changed state")
			}
		})
	}
	fixture := newFactorFixture(FactorProfile{ID: "owner"})
	if err := fixture.transaction(time.Now).Enable("missing", "secret", []string{"code"}); !errors.Is(err, ErrProfileNotFound) || fixture.writes != 0 {
		t.Fatalf("missing profile result = %v, writes %d", err, fixture.writes)
	}
	var missing *FactorTransaction[factorRecord]
	if err := missing.Enable("owner", "secret", []string{"code"}); !errors.Is(err, ErrInvalidFactorInput) {
		t.Fatalf("nil transaction error = %v", err)
	}
}

func TestFactorEnableAcceptsExactBounds(t *testing.T) {
	t.Parallel()
	id, secret := strings.Repeat("i", maxIdentityLength), strings.Repeat("s", maxSecretLength)
	recovery := make([]string, recoveryCodeCount)
	for index := range recovery {
		recovery[index] = strings.Repeat(string(rune('a'+index)), maxSecretLength)
	}
	fixture := newFactorFixture(FactorProfile{ID: id})
	if err := fixture.transaction(time.Now).Enable(id, secret, recovery); err != nil {
		t.Fatal(err)
	}
	if len(fixture.profiles[0].factor.Recovery) != recoveryCodeCount || fixture.profiles[0].factor.Secret != secret {
		t.Fatalf("bounded factor = %#v", fixture.profiles[0].factor)
	}
}

func TestFactorDisablePreservesOwnerInvariantAndCommits(t *testing.T) { //nolint:cyclop // One transaction test proves the owner invariant and every committed field.
	t.Parallel()
	blocked := newFactorFixture(FactorProfile{ID: "owner", Owner: true, Secret: "secret", Recovery: []string{"code"}, Revision: 7})
	if err := blocked.transaction(time.Now).Disable("owner"); !errors.Is(err, ErrOwnerFactorNeeded) || fixtureChanged(blocked) {
		t.Fatalf("blocked disable = %v, writes %d", err, blocked.writes)
	}
	fixture := newFactorFixture(FactorProfile{ID: "owner", Owner: true, Passkeys: 1, Secret: "secret", Recovery: []string{"code"}, Revision: 7})
	if err := fixture.transaction(time.Now).Disable("owner"); err != nil {
		t.Fatal(err)
	}
	factor := fixture.profiles[0].factor
	if factor.Secret != "" || factor.Recovery != nil || factor.Revision != 8 || fixture.writes != 1 || !fixture.related || !fixture.atomic {
		t.Fatalf("disabled factor = %#v, writes %d", factor, fixture.writes)
	}
	if _, found := fixture.sessions["public"]; found {
		t.Fatal("public session survived factor disable")
	}
	for _, id := range []string{"", " owner", strings.Repeat("i", maxIdentityLength+1), "owner\x00", "missing"} {
		invalid := newFactorFixture(FactorProfile{ID: "owner", Passkeys: 1})
		if err := invalid.transaction(time.Now).Disable(id); !errors.Is(err, ErrProfileNotFound) || invalid.writes != 0 {
			t.Fatalf("disable %q = %v, writes %d", id, err, invalid.writes)
		}
	}
}

func fixtureChanged(fixture *factorFixture) bool {
	return fixture.writes != 0 || fixture.profiles[0].factor.Secret != "secret" || len(fixture.sessions) != 3
}

func TestFactorPersistenceFailureDoesNotCommit(t *testing.T) {
	t.Parallel()
	persistErr := errors.New("persist failed")
	tests := []struct {
		name string
		run  func(*FactorTransaction[factorRecord]) error
	}{
		{"enable", func(transaction *FactorTransaction[factorRecord]) error {
			return transaction.Enable("owner", "new-secret", []string{"new-code"})
		}},
		{"disable", func(transaction *FactorTransaction[factorRecord]) error { return transaction.Disable("owner") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFactorFixture(FactorProfile{ID: "owner", Passkeys: 1, Secret: "secret", Recovery: []string{"code"}, Revision: 3})
			fixture.fail = persistErr
			beforeProfiles, beforeSessions := cloneFactorRecords(fixture.profiles), CloneSessions(fixture.sessions)
			if err := test.run(fixture.transaction(time.Now)); !errors.Is(err, persistErr) {
				t.Fatalf("error = %v", err)
			}
			if fixture.writes != 1 || !fixture.atomic || !reflect.DeepEqual(fixture.profiles, beforeProfiles) || !reflect.DeepEqual(fixture.sessions, beforeSessions) {
				t.Fatal("failed persistence changed memory")
			}
		})
	}
}

func TestFactorVerifyAcceptsTOTPAndConsumesRecoveryOnce(t *testing.T) { //nolint:cyclop // One verification lifecycle proves TOTP and one-use recovery behavior.
	t.Parallel()
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // Public RFC 6238 test key.
	totpFixture := newFactorFixture(FactorProfile{ID: "owner", Secret: secret, Recovery: make([]string, recoveryCodeCount+1)})
	if !totpFixture.transaction(func() time.Time { return time.Unix(59, 0) }).Verify("owner", "287082") || totpFixture.writes != 0 {
		t.Fatal("valid TOTP was not accepted without persistence")
	}
	recoveryFixture := newFactorFixture(FactorProfile{ID: "owner", Secret: secret, Recovery: []string{SessionKey("AABBCCDD"), SessionKey("unused")}})
	transaction := recoveryFixture.transaction(func() time.Time { return time.Unix(59, 0) })
	if !transaction.Verify("owner", " aa-bb cc-dd ") {
		t.Fatal("valid recovery code was rejected")
	}
	if recoveryFixture.writes != 1 || recoveryFixture.related || !recoveryFixture.atomic || !reflect.DeepEqual(recoveryFixture.profiles[0].factor.Recovery, []string{SessionKey("unused")}) {
		t.Fatalf("recovery commit = writes %d related %t factor %#v", recoveryFixture.writes, recoveryFixture.related, recoveryFixture.profiles[0].factor)
	}
	if len(recoveryFixture.sessions) != 3 || !reflect.DeepEqual(recoveryFixture.persistedSessions, recoveryFixture.sessions) {
		t.Fatal("recovery verification changed sessions")
	}
	if transaction.Verify("owner", " aa-bb cc-dd ") || recoveryFixture.writes != 1 {
		t.Fatal("recovery code was reusable")
	}
}

func TestFactorVerifyRejectsInvalidStateWithoutPersistence(t *testing.T) { //nolint:gocognit // One table proves all verification trust-boundary failures are side-effect free.
	t.Parallel()
	tests := []struct {
		name    string
		profile FactorProfile
		id      string
		code    string
	}{
		{"missing identity", FactorProfile{ID: "owner", Secret: "secret"}, "", "code"},
		{"spaced identity", FactorProfile{ID: "owner", Secret: "secret"}, " owner", "code"},
		{"oversized identity", FactorProfile{ID: "owner", Secret: "secret"}, strings.Repeat("i", maxIdentityLength+1), "code"},
		{"nul identity", FactorProfile{ID: "owner", Secret: "secret"}, "owner\x00", "code"},
		{"missing code", FactorProfile{ID: "owner", Secret: "secret"}, "owner", ""},
		{"oversized code", FactorProfile{ID: "owner", Secret: "secret"}, "owner", strings.Repeat("c", maxSecretLength+1)},
		{"nul code", FactorProfile{ID: "owner", Secret: "secret"}, "owner", "code\x00"},
		{"missing profile", FactorProfile{ID: "owner", Secret: "secret"}, "missing", "code"},
		{"missing factor", FactorProfile{ID: "owner"}, "owner", "code"},
		{"too many stored recovery codes", FactorProfile{ID: "owner", Secret: "secret", Recovery: make([]string, recoveryCodeCount+1)}, "owner", "code"},
		{"empty normalized code", FactorProfile{ID: "owner", Secret: "secret"}, "owner", " -- "},
		{"wrong recovery", FactorProfile{ID: "owner", Secret: "secret", Recovery: []string{SessionKey("right")}}, "owner", "wrong"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFactorFixture(test.profile)
			beforeProfiles, beforeSessions := cloneFactorRecords(fixture.profiles), CloneSessions(fixture.sessions)
			if fixture.transaction(time.Now).Verify(test.id, test.code) || fixture.writes != 0 || !reflect.DeepEqual(fixture.profiles, beforeProfiles) || !reflect.DeepEqual(fixture.sessions, beforeSessions) {
				t.Fatal("invalid verification changed state")
			}
		})
	}
	var missing *FactorTransaction[factorRecord]
	if missing.Verify("owner", "code") {
		t.Fatal("nil transaction verified a factor")
	}
}

func TestFactorRecoveryPersistenceFailureDoesNotConsumeCode(t *testing.T) {
	t.Parallel()
	fixture := newFactorFixture(FactorProfile{ID: "owner", Secret: "secret", Recovery: []string{SessionKey("CODE")}})
	fixture.fail = errors.New("persist failed")
	before := cloneFactorRecords(fixture.profiles)
	if fixture.transaction(time.Now).Verify("owner", "code") || fixture.writes != 1 || !fixture.atomic || !reflect.DeepEqual(fixture.profiles, before) {
		t.Fatal("failed recovery persistence consumed the code")
	}
}
