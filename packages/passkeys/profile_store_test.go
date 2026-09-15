package passkeys

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

type storeProfile struct {
	ID          string
	Name        string
	Owner       bool
	Alternative bool
	Credentials []webauthn.Credential
	Usage       map[string]Usage
	Revision    int
}

type storeSession struct{ ProfileID string }

type storeFixture struct {
	mu              sync.RWMutex
	profiles        []storeProfile
	sessions        map[string]storeSession
	keys            map[string]string
	saveProfiles    int
	saveRelated     int
	failProfiles    error
	failRelated     error
	removedSessions int
}

func newStoreFixture(profiles ...storeProfile) (*storeFixture, *ProfileStore[storeProfile, storeSession, string]) {
	fixture := &storeFixture{
		profiles: profiles,
		sessions: map[string]storeSession{"public": {ProfileID: "profile"}, "other": {ProfileID: "other"}},
		keys:     map[string]string{"key": "value"},
	}
	config := ProfileStoreConfig[storeProfile, storeSession, string]{
		Mutex: &fixture.mu, Profiles: &fixture.profiles, Sessions: &fixture.sessions, Keys: &fixture.keys,
		Access: ProfileAccess[storeProfile]{
			ID: func(profile *storeProfile) string { return profile.ID }, Name: func(profile *storeProfile) string { return profile.Name },
			Credentials: func(profile *storeProfile) *[]webauthn.Credential { return &profile.Credentials }, Usage: func(profile *storeProfile) *map[string]Usage { return &profile.Usage },
			Owner: func(profile *storeProfile) bool { return profile.Owner }, AlternativeFactor: func(profile *storeProfile) bool { return profile.Alternative },
			IncrementRevision: func(profile *storeProfile) { profile.Revision++ }, Clone: cloneStoreProfiles,
		},
		SaveProfiles: func(_ []storeProfile) error { fixture.saveProfiles++; return fixture.failProfiles },
		SaveRelated: func(_ []storeProfile, _ map[string]storeSession, _ map[string]string) error {
			fixture.saveRelated++
			return fixture.failRelated
		},
		WithoutPublicSessions: func(sessions map[string]storeSession, profileID string) map[string]storeSession {
			result := make(map[string]storeSession, len(sessions))
			for key, session := range sessions {
				if session.ProfileID == profileID {
					fixture.removedSessions++
					continue
				}
				result[key] = session
			}
			return result
		},
		Now: func() time.Time { return time.Unix(500, 0) },
	}
	return fixture, NewProfileStore(config)
}

func cloneStoreProfiles(profiles []storeProfile) []storeProfile {
	result := make([]storeProfile, len(profiles))
	for index, profile := range profiles {
		result[index] = profile
		result[index].Credentials = CloneAll(profile.Credentials)
		if profile.Usage != nil {
			result[index].Usage = make(map[string]Usage, len(profile.Usage))
			for key, usage := range profile.Usage {
				result[index].Usage[key] = usage
			}
		}
	}
	return result
}

func TestProfileStoreDiscoverValidatesAndIsolates(t *testing.T) {
	original := credential("first")
	fixture, store := newStoreFixture(storeProfile{ID: "profile", Name: "Viewer", Credentials: []webauthn.Credential{original}})
	for _, lookup := range []struct{ raw, handle []byte }{{nil, []byte("profile")}, {[]byte("first"), nil}, {[]byte("missing"), []byte("profile")}, {[]byte("first"), []byte("missing")}} {
		if _, err := store.Discover(lookup.raw, lookup.handle); err == nil {
			t.Fatalf("invalid lookup accepted: %#v", lookup)
		}
	}
	user, err := store.Discover([]byte("first"), []byte("profile"))
	if err != nil {
		t.Fatal(err)
	}
	found := user.(User[storeProfile])
	found.Credentials[0].ID[0] = 'X'
	if string(fixture.profiles[0].Credentials[0].ID) != "first" || found.Name != "Viewer" {
		t.Fatalf("discover aliases storage or loses identity: %#v", found)
	}
	if _, err := (&ProfileStore[storeProfile, storeSession, string]{}).Discover([]byte("first"), []byte("profile")); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("invalid store = %v", err)
	}
}

func TestProfileStoreAddRejectsBeforeEffectsAndCommitsAtomically(t *testing.T) { //nolint:cyclop // One atomicity scenario covers every rejected and committed effect.
	first, second := credential("first"), credential("second")
	fixture, store := newStoreFixture(storeProfile{ID: "profile", Credentials: []webauthn.Credential{first}})
	for _, test := range []struct {
		profile string
		value   *webauthn.Credential
		want    error
	}{{"profile", nil, ErrInvalidCredential}, {"missing", &second, ErrNotFound}, {"profile", &first, ErrDuplicate}} {
		if err := store.Add(test.profile, test.value); !errors.Is(err, test.want) {
			t.Fatalf("Add() = %v, want %v", err, test.want)
		}
	}
	if fixture.saveRelated != 0 || fixture.removedSessions != 0 || len(fixture.profiles[0].Credentials) != 1 {
		t.Fatal("rejected addition caused effects")
	}
	fixture.failRelated = errors.New("disk unavailable")
	if err := store.Add("profile", &second); !errors.Is(err, fixture.failRelated) {
		t.Fatalf("save failure = %v", err)
	}
	if len(fixture.profiles[0].Credentials) != 1 || fixture.profiles[0].Revision != 0 || len(fixture.sessions) != 2 {
		t.Fatal("failed save mutated state")
	}
	fixture.failRelated = nil
	if err := store.Add("profile", &second); err != nil {
		t.Fatal(err)
	}
	id := ID(second.ID)
	if len(fixture.profiles[0].Credentials) != 2 || !fixture.profiles[0].Usage[id].Tracked || fixture.profiles[0].Revision != 1 || len(fixture.sessions) != 1 || fixture.saveRelated != 2 {
		t.Fatalf("committed state = %#v %#v", fixture.profiles, fixture.sessions)
	}
	second.ID[0] = 'X'
	if string(fixture.profiles[0].Credentials[1].ID) != "second" {
		t.Fatal("addition aliases caller memory")
	}
}

func TestProfileStoreAddRejectsGlobalDuplicateAndLimit(t *testing.T) {
	duplicate := credential("duplicate")
	fixture, store := newStoreFixture(
		storeProfile{ID: "profile"},
		storeProfile{ID: "other", Credentials: []webauthn.Credential{duplicate}},
	)
	if err := store.Add("profile", &duplicate); !errors.Is(err, ErrDuplicate) || fixture.saveRelated != 0 {
		t.Fatalf("global duplicate = %v, effects %d", err, fixture.saveRelated)
	}
	full := make([]webauthn.Credential, MaxCredentials)
	for index := range full {
		full[index] = credential(string(rune(index + 1)))
	}
	fixture, store = newStoreFixture(storeProfile{ID: "profile", Credentials: full})
	addition := credential("additional")
	if err := store.Add("profile", &addition); !errors.Is(err, ErrLimit) || fixture.saveRelated != 0 {
		t.Fatalf("limit = %v, effects %d", err, fixture.saveRelated)
	}
}

func TestProfileStoreUpdateRejectsAndRollsBackBeforeCommit(t *testing.T) { //nolint:cyclop // One atomicity scenario covers validation, rollback, and commit.
	first := credential("first")
	fixture, store := newStoreFixture(storeProfile{ID: "profile", Credentials: []webauthn.Credential{first}})
	unknown := credential("unknown")
	for _, test := range []struct {
		profile string
		value   *webauthn.Credential
	}{{"profile", nil}, {"missing", &first}, {"profile", &unknown}} {
		if err := store.Update(test.profile, test.value); err == nil {
			t.Fatal("invalid update accepted")
		}
	}
	if fixture.saveProfiles != 0 {
		t.Fatal("rejected update saved")
	}
	updated := Clone(first)
	updated.Authenticator.SignCount = 7
	fixture.failProfiles = errors.New("disk unavailable")
	if err := store.Update("profile", &updated); !errors.Is(err, fixture.failProfiles) || fixture.profiles[0].Credentials[0].Authenticator.SignCount != 0 {
		t.Fatalf("failed update = %v, state %#v", err, fixture.profiles)
	}
	fixture.failProfiles = nil
	if err := store.Update("profile", &updated); err != nil {
		t.Fatal(err)
	}
	usage := fixture.profiles[0].Usage[ID(first.ID)]
	if fixture.profiles[0].Credentials[0].Authenticator.SignCount != 7 || usage.LastUsed != 500 || !usage.Tracked || fixture.saveProfiles != 2 {
		t.Fatalf("updated state = %#v", fixture.profiles[0])
	}
}

func TestProfileStoreInventoryAndRemove(t *testing.T) { //nolint:cyclop,gocognit // Removal invariants share one stateful fixture.
	first, second := credential("first"), credential("second")
	firstID, secondID := ID(first.ID), ID(second.ID)
	fixture, store := newStoreFixture(storeProfile{ID: "profile", Owner: true, Credentials: []webauthn.Credential{first, second}, Usage: map[string]Usage{firstID: {Tracked: true}, secondID: {Tracked: true}}})
	if got := store.Inventory("missing"); got == nil || len(got) != 0 {
		t.Fatalf("missing inventory = %#v", got)
	}
	if got := store.Inventory("profile"); len(got) != 2 || got[0].ID != firstID {
		t.Fatalf("inventory = %#v", got)
	}
	if err := store.Remove("profile", "invalid"); !errors.Is(err, ErrInvalidID) || fixture.saveRelated != 0 {
		t.Fatalf("invalid removal = %v", err)
	}
	if err := store.Remove("missing", firstID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing profile = %v", err)
	}
	fixture.failRelated = errors.New("disk unavailable")
	if err := store.Remove("profile", firstID); !errors.Is(err, fixture.failRelated) || len(fixture.profiles[0].Credentials) != 2 {
		t.Fatalf("failed removal = %v, state %#v", err, fixture.profiles)
	}
	fixture.failRelated = nil
	if err := store.Remove("profile", firstID); err != nil {
		t.Fatal(err)
	}
	if len(fixture.profiles[0].Credentials) != 1 || fixture.profiles[0].Usage[firstID].Tracked || fixture.profiles[0].Revision != 1 || len(fixture.sessions) != 1 {
		t.Fatalf("removed state = %#v %#v", fixture.profiles, fixture.sessions)
	}
	if err := store.Remove("profile", firstID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed credential found = %v", err)
	}
	if err := store.Remove("profile", secondID); !errors.Is(err, ErrLastFactor) || fixture.saveRelated != 2 {
		t.Fatalf("last factor = %v, effects %d", err, fixture.saveRelated)
	}
	fixture.profiles[0].Alternative = true
	if err := store.Remove("profile", secondID); err != nil || fixture.profiles[0].Usage != nil || len(fixture.profiles[0].Credentials) != 0 {
		t.Fatalf("alternative-factor removal = %v, state %#v", err, fixture.profiles)
	}
}

func TestProfileStoreInvalidConfigHasNoEffects(t *testing.T) {
	store := NewProfileStore(ProfileStoreConfig[storeProfile, storeSession, string]{})
	item := credential("first")
	if store.valid() || store.Inventory("profile") == nil || !errors.Is(store.Add("profile", &item), ErrInvalidCredential) || !errors.Is(store.Update("profile", &item), ErrInvalidCredential) || !errors.Is(store.Remove("profile", ID(item.ID)), ErrInvalidID) {
		t.Fatal("invalid configuration was accepted")
	}
	var nilStore *ProfileStore[storeProfile, storeSession, string]
	if nilStore.valid() {
		t.Fatal("nil store was valid")
	}
}
