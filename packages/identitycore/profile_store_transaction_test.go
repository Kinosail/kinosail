package identitycore

import (
	"errors"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
)

type profileStoreFixture struct {
	mutex    sync.RWMutex
	profiles []Profile
	sessions map[string]Session
	keys     map[string]APIKey
	failFile string
	writes   []string
}

func newProfileStoreFixture(profiles ...Profile) (*profileStoreFixture, ProfileStore) {
	fixture := &profileStoreFixture{profiles: profiles, sessions: map[string]Session{}, keys: map[string]APIKey{}}
	persistence := NewProfilePersistence(nil, false, func(path string, _ any) error {
		fixture.writes = append(fixture.writes, path)
		if path == fixture.failFile {
			return errors.New("persist failed")
		}
		return nil
	}, "profiles", "sessions", "keys")
	store := NewProfileStore(ProfileStoreConfig{
		Mutex: &fixture.mutex, Profiles: &fixture.profiles, Sessions: &fixture.sessions, APIKeys: &fixture.keys,
		Persistence: persistence,
		Create: func(name, _ string, owner bool) (Profile, error) {
			if name == "fail" {
				return Profile{}, errors.New("create failed")
			}
			return Profile{ID: "new", Name: name, Owner: owner, Revision: 1}, nil
		},
	})
	return fixture, store
}

func TestProfileStoreSetupAndAuthentication(t *testing.T) { //nolint:cyclop // The score of 13 remains below the repository ceiling of 22 for one setup/authentication matrix.
	fixture, store := newProfileStoreFixture()
	if store.HasProfiles() {
		t.Fatal("empty store reports profiles")
	}
	owner, err := NewProfile("Owner", "owner-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddOwner(owner); err != nil || !store.HasProfiles() {
		t.Fatalf("add owner: %v", err)
	}
	if profile, valid := store.Authenticate(" owner ", "owner-password"); !valid || profile.ID != owner.ID {
		t.Fatalf("authenticate = %#v, %v", profile, valid)
	}
	if _, valid := store.Authenticate("Owner", "wrong-password"); valid {
		t.Fatal("wrong password authenticated")
	}
	fixture.profiles[0].Disabled = true
	if _, valid := store.Authenticate("Owner", "owner-password"); valid {
		t.Fatal("disabled profile authenticated")
	}
	fixture.profiles[0].Disabled, fixture.profiles[0].SCIMDeleted = false, true
	if _, valid := store.Authenticate("missing", "owner-password"); valid {
		t.Fatal("missing profile authenticated")
	}
	if err := store.AddOwner(owner); err == nil {
		t.Fatal("second owner was accepted")
	}

	failing, failingStore := newProfileStoreFixture()
	failing.failFile = "profiles"
	if err := failingStore.AddOwner(owner); err == nil || len(failing.profiles) != 0 {
		t.Fatalf("failed owner commit changed state: %v", err)
	}
}

func TestProfileStoreAddProfileBoundaries(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the add-profile boundary matrix.
	fixture, store := newProfileStoreFixture(Profile{ID: "owner", Name: "Owner", Owner: true})
	if _, err := store.AddProfile("Viewer", "password-password", false, ProfilePolicy{AccessStart: "10:00"}); err == nil {
		t.Fatal("invalid policy was accepted")
	}
	if _, err := store.AddProfile("fail", "password-password", false, ProfilePolicy{}); err == nil {
		t.Fatal("create failure was ignored")
	}
	if _, err := store.AddProfile("owner", "password-password", false, ProfilePolicy{}); err == nil {
		t.Fatal("duplicate name was accepted")
	}
	fixture.failFile = "profiles"
	if _, err := store.AddProfile("Viewer", "password-password", false, ProfilePolicy{}); err == nil || len(fixture.profiles) != 1 {
		t.Fatalf("failed persist changed state: %v", err)
	}
	fixture.failFile = ""
	id, err := store.AddProfile("Viewer", "password-password", false, ProfilePolicy{Remote: true, Rating: "teen", Libraries: []string{"Films"}})
	if err != nil || id != "new" || len(fixture.profiles) != 2 || !fixture.profiles[1].Remote || fixture.profiles[1].Rating != "teen" {
		t.Fatalf("add profile: id=%q err=%v profiles=%#v", id, err, fixture.profiles)
	}

	defaultStore := NewProfileStore(ProfileStoreConfig{Mutex: &fixture.mutex, Profiles: &fixture.profiles, Sessions: &fixture.sessions, APIKeys: &fixture.keys})
	if defaultStore.config.Create == nil {
		t.Fatal("default profile factory was not installed")
	}
}

func TestProfileStoreSetProfileTransactions(t *testing.T) { //nolint:cyclop // The score of 17 remains below the repository ceiling of 22 for the atomic update matrix.
	fixture, store := newProfileStoreFixture(
		Profile{ID: "owner", Name: "Owner", Owner: true, Revision: 1},
		Profile{ID: "viewer", Name: "Viewer", Revision: 2},
	)
	fixture.sessions = map[string]Session{
		"public": {ProfileID: "viewer", Channel: "public"},
		"local":  {ProfileID: "viewer"},
	}
	if err := store.SetProfile("viewer", false, ProfilePolicy{AccessEnd: "10:00"}); err == nil {
		t.Fatal("invalid policy was accepted")
	}
	if err := store.SetProfile("missing", false, ProfilePolicy{}); err == nil {
		t.Fatal("missing profile was updated")
	}
	if err := store.SetProfile("owner", false, ProfilePolicy{}); err == nil {
		t.Fatal("last owner was demoted")
	}
	fixture.failFile = "profiles"
	if err := store.SetProfile("viewer", false, ProfilePolicy{Remote: true}); err == nil || fixture.profiles[1].Revision != 2 {
		t.Fatalf("failed remote update changed state: %v", err)
	}
	fixture.failFile = ""
	if err := store.SetProfile("viewer", false, ProfilePolicy{Remote: true}); err != nil || fixture.profiles[1].Revision != 3 || len(fixture.sessions) != 2 {
		t.Fatalf("remote update: %v state=%#v", err, fixture)
	}
	for _, failedFile := range []string{"sessions", "keys", "profiles"} {
		fixture.failFile = failedFile
		before := fixture.profiles[1].Revision
		if err := store.SetProfile("viewer", false, ProfilePolicy{}); err == nil || fixture.profiles[1].Revision != before || len(fixture.sessions) != 2 {
			t.Fatalf("%s failure changed state: %v", failedFile, err)
		}
	}
	fixture.failFile = ""
	if err := store.SetProfile("viewer", false, ProfilePolicy{}); err != nil || fixture.profiles[1].Revision != 4 {
		t.Fatal(err)
	}
	if _, found := fixture.sessions["public"]; found || fixture.sessions["local"].ProfileID != "viewer" {
		t.Fatalf("wrong session revocation: %#v", fixture.sessions)
	}
}

func TestProfileStoreRemoveAndSnapshots(t *testing.T) { //nolint:cyclop // The score of 16 remains below the repository ceiling of 22 for the removal/snapshot matrix.
	fixture, store := newProfileStoreFixture(
		Profile{ID: "owner", Name: "Owner", Owner: true},
		Profile{ID: "viewer", Name: "Viewer", Libraries: []string{"Films"}},
		Profile{ID: "scim", Name: "SCIM", SCIMManaged: true},
	)
	fixture.sessions = map[string]Session{"viewer": {ProfileID: "viewer"}, "owner": {ProfileID: "owner"}}
	fixture.keys = map[string]APIKey{"viewer": {ProfileID: "viewer", Scopes: []string{"stream"}}, "owner": {ProfileID: "owner"}}
	if err := store.RemoveProfile("scim"); err == nil {
		t.Fatal("SCIM profile was removed")
	}
	if err := store.RemoveProfile("owner"); err == nil {
		t.Fatal("last owner was removed")
	}
	if err := store.RemoveProfile("missing"); err == nil {
		t.Fatal("missing profile was removed")
	}
	fixture.failFile = "sessions"
	if err := store.RemoveProfile("viewer"); err == nil || len(fixture.profiles) != 3 {
		t.Fatalf("failed removal changed profiles: %v", err)
	}
	fixture.failFile = ""
	if err := store.RemoveProfile("viewer"); err != nil {
		t.Fatal(err)
	}
	if len(fixture.profiles) != 2 || len(fixture.sessions) != 1 || len(fixture.keys) != 1 {
		t.Fatalf("related state was not removed: %#v", fixture)
	}
	profile, found := store.ByID("owner")
	if !found || profile.ID != "owner" {
		t.Fatalf("ByID = %#v, %v", profile, found)
	}
	if _, found := store.ByID("missing"); found {
		t.Fatal("missing profile found")
	}
	listed := store.List()
	listed[0].Name = "Changed"
	if fixture.profiles[0].Name == "Changed" {
		t.Fatal("List returned shared state")
	}

	federated := store.FederatedProfiles()
	if err := federated.Link("OIDC", "owner", federationIdentity()); err != nil {
		t.Fatal(err)
	}
	if fixture.profiles[0].OIDCSubject == "" {
		t.Fatal("federated commit did not update profiles")
	}
}

func federationIdentity() federation.Identity {
	return federation.Identity{Issuer: "https://issuer.example", Subject: "subject"}
}
