package identitycore

import (
	"errors"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/scim"
)

var scimTestNow = time.Date(2026, time.September, 4, 12, 0, 0, 0, time.FixedZone("test", -6*60*60))

func scimInput(name, userName string, active bool) scim.ProfileInput {
	return scim.ProfileInput{
		Name: name, UserName: userName, Active: active,
		Formatted: name, GivenName: "Test", FamilyName: "Viewer", ExternalID: "directory-id",
		Emails:     []scim.Email{{Value: userName, Type: "work", Primary: true}},
		Enterprise: scim.EnterpriseProfile{Department: "Media"},
	}
}

func configureSCIMStore(store ProfileStore, ids ...string) ProfileStore {
	store.config.Now = func() time.Time { return scimTestNow }
	index := 0
	store.config.NewID = func() string {
		value := ids[index]
		index++
		return value
	}
	return store
}

func TestProfileStoreSCIMReadAndCreate(t *testing.T) { //nolint:cyclop,gocognit // The read/create matrix remains below the repository ceiling of 22.
	fixture, store := newProfileStoreFixture(
		Profile{ID: "owner", Name: "Owner", Owner: true},
		Profile{ID: "active", Name: "Active", SCIMManaged: true, SCIMUserName: "active@example.com"},
		Profile{ID: "deleted", Name: "Deleted", SCIMManaged: true, SCIMDeleted: true},
		Profile{ID: "local", Name: "Local"},
	)
	store = configureSCIMStore(store, "active", "created")
	if profiles := store.SCIMProfiles(); len(profiles) != 1 || profiles[0].ID != "active" {
		t.Fatalf("SCIMProfiles = %#v", profiles)
	}
	if profile, found := store.SCIMProfile("active"); !found || profile.ID != "active" {
		t.Fatalf("SCIMProfile = %#v, %v", profile, found)
	}
	for _, id := range []string{"deleted", "local", "missing"} {
		if _, found := store.SCIMProfile(id); found {
			t.Fatalf("inactive %q was returned", id)
		}
	}
	if _, err := store.CreateSCIMProfile(scim.ProfileInput{}); err == nil || len(fixture.writes) != 0 {
		t.Fatalf("invalid input caused side effects: %v %#v", err, fixture.writes)
	}

	noOwner, noOwnerStore := newProfileStoreFixture()
	noOwnerStore = configureSCIMStore(noOwnerStore, "unused")
	if _, err := noOwnerStore.CreateSCIMProfile(scimInput("Viewer", "viewer@example.com", true)); !errors.Is(err, scim.ErrSetupRequired) || len(noOwner.writes) != 0 {
		t.Fatalf("setup invariant = %v writes=%#v", err, noOwner.writes)
	}

	if _, err := store.CreateSCIMProfile(scimInput("Owner", "new@example.com", true)); !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("name conflict = %v", err)
	}
	if _, err := store.CreateSCIMProfile(scimInput("Other", "active@example.com", true)); !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("userName conflict = %v", err)
	}
	created, err := store.CreateSCIMProfile(scimInput("Created", "created@example.com", false))
	if err != nil || created.ID != "created" || !created.Disabled || created.Rating != "family" || !created.SCIMCreatedAt.Equal(scimTestNow.UTC()) {
		t.Fatalf("created = %#v, %v", created, err)
	}
	if len(fixture.profiles) != 5 {
		t.Fatalf("create did not commit: %#v", fixture.profiles)
	}
}

func TestProfileStoreSCIMCreateFailuresAndRehydrate(t *testing.T) { //nolint:cyclop // The score of 16 remains below the repository ceiling of 22 for the failure matrix.
	deleted := Profile{
		ID: "deleted", Name: "Old", Credential: "credential", OIDCIssuer: "issuer", OIDCSubject: "subject",
		SAMLIssuer: "issuer", SAMLSubject: "subject", TOTPSecret: "totp", Recovery: []string{"code"},
		SCIMManaged: true, SCIMDeleted: true, Disabled: true, SCIMUserName: "viewer@example.com", SCIMExternalID: "directory-id", Revision: 4,
	}
	fixture, store := newProfileStoreFixture(Profile{ID: "owner", Name: "Owner", Owner: true}, deleted)
	store = configureSCIMStore(store, "new")
	fixture.sessions["session"] = Session{ProfileID: "deleted"}
	fixture.failFile = "sessions"
	if _, err := store.CreateSCIMProfile(scimInput("Viewer", "viewer@example.com", true)); err == nil || fixture.profiles[1].SCIMDeleted == false || len(fixture.sessions) != 1 {
		t.Fatalf("failed rehydrate changed state: %v %#v", err, fixture)
	}
	fixture.failFile = ""
	rehydrated, err := store.CreateSCIMProfile(scimInput("Viewer", "viewer@example.com", true))
	if err != nil || rehydrated.SCIMDeleted || rehydrated.Disabled || rehydrated.Credential != "" || rehydrated.OIDCSubject != "" || rehydrated.SAMLSubject != "" || rehydrated.TOTPSecret != "" || rehydrated.Revision != 5 || len(fixture.sessions) != 0 {
		t.Fatalf("rehydrate = %#v, %v fixture=%#v", rehydrated, err, fixture)
	}

	_, duplicateStore := newProfileStoreFixture(
		Profile{ID: "owner", Name: "Owner", Owner: true},
		Profile{ID: "one", Name: "One", SCIMManaged: true, SCIMDeleted: true, SCIMUserName: "same@example.com"},
		Profile{ID: "two", Name: "Two", SCIMManaged: true, SCIMDeleted: true, SCIMUserName: "same@example.com"},
	)
	duplicateStore = configureSCIMStore(duplicateStore, "unused")
	if _, err := duplicateStore.CreateSCIMProfile(scimInput("Same", "same@example.com", true)); !errors.Is(err, scim.ErrConflict) {
		t.Fatalf("duplicate tombstone = %v", err)
	}

	failing, failingStore := newProfileStoreFixture(Profile{ID: "owner", Name: "Owner", Owner: true})
	failingStore = configureSCIMStore(failingStore, "new")
	failing.failFile = "sessions"
	if _, err := failingStore.CreateSCIMProfile(scimInput("New", "new@example.com", true)); err == nil || len(failing.profiles) != 1 {
		t.Fatalf("failed create changed state: %v", err)
	}
}

func TestProfileStoreSCIMUpdate(t *testing.T) { //nolint:cyclop,gocognit // The update matrix remains below the repository ceiling of 22.
	fixture, store := newProfileStoreFixture(
		Profile{ID: "owner", Name: "Owner", Owner: true},
		Profile{ID: "target", Name: "Target", SCIMManaged: true, SCIMUserName: "target@example.com", SCIMExternalID: "directory-id", Revision: 2},
		Profile{ID: "other", Name: "Other", SCIMManaged: true, SCIMUserName: "other@example.com"},
		Profile{ID: "deleted", Name: "Deleted", SCIMManaged: true, SCIMDeleted: true},
		Profile{ID: "local", Name: "Local"},
	)
	store = configureSCIMStore(store, "unused")
	fixture.sessions["target"] = Session{ProfileID: "target"}
	if _, err := store.UpdateSCIMProfile("target", scim.ProfileInput{}, ""); err == nil || len(fixture.writes) != 0 {
		t.Fatalf("invalid update caused side effects: %v", err)
	}
	for _, id := range []string{"missing", "deleted", "local"} {
		if _, err := store.UpdateSCIMProfile(id, scimInput("New", "new@example.com", true), ""); !errors.Is(err, scim.ErrProfileNotFound) {
			t.Fatalf("update %q = %v", id, err)
		}
	}
	if _, err := store.UpdateSCIMProfile("target", scimInput("Target", "target@example.com", true), `W/"1"`); !errors.Is(err, scim.ErrPrecondition) {
		t.Fatalf("precondition = %v", err)
	}
	for _, input := range []scim.ProfileInput{scimInput("Other", "new@example.com", true), scimInput("New", "other@example.com", true)} {
		if _, err := store.UpdateSCIMProfile("target", input, ""); !errors.Is(err, scim.ErrConflict) {
			t.Fatalf("conflict = %v", err)
		}
	}
	fixture.failFile = "sessions"
	if _, err := store.UpdateSCIMProfile("target", scimInput("Updated", "updated@example.com", true), `W/"2"`); err == nil || fixture.profiles[1].Revision != 2 {
		t.Fatalf("failed update changed state: %v", err)
	}
	fixture.failFile = ""
	updated, err := store.UpdateSCIMProfile("target", scimInput("Updated", "updated@example.com", true), `W/"2"`)
	if err != nil || updated.Revision != 3 || len(fixture.sessions) != 1 {
		t.Fatalf("active update = %#v, %v", updated, err)
	}
	updated, err = store.UpdateSCIMProfile("target", scimInput("Updated", "updated@example.com", false), "")
	if err != nil || !updated.Disabled || len(fixture.sessions) != 0 {
		t.Fatalf("disable update = %#v, %v", updated, err)
	}
	fixture.sessions["disabled"] = Session{ProfileID: "target"}
	if _, err := store.UpdateSCIMProfile("target", scimInput("Updated", "updated@example.com", true), ""); err != nil || len(fixture.sessions) != 0 {
		t.Fatalf("reenable did not revoke old sessions: %v", err)
	}
}

func TestProfileStoreSCIMDelete(t *testing.T) { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for the delete transaction matrix.
	fixture, store := newProfileStoreFixture(
		Profile{ID: "target", Name: "Target", SCIMManaged: true, SCIMUserName: "target@example.com", Revision: 2},
		Profile{ID: "deleted", SCIMManaged: true, SCIMDeleted: true},
		Profile{ID: "local"},
	)
	store = configureSCIMStore(store, "unused")
	fixture.sessions["target"] = Session{ProfileID: "target"}
	for _, id := range []string{"missing", "deleted", "local"} {
		if err := store.DeleteSCIMProfile(id, ""); !errors.Is(err, scim.ErrProfileNotFound) {
			t.Fatalf("delete %q = %v", id, err)
		}
	}
	if err := store.DeleteSCIMProfile("target", `W/"1"`); !errors.Is(err, scim.ErrPrecondition) {
		t.Fatalf("precondition = %v", err)
	}
	fixture.failFile = "sessions"
	if err := store.DeleteSCIMProfile("target", `W/"2"`); err == nil || fixture.profiles[0].SCIMDeleted {
		t.Fatalf("failed delete changed state: %v", err)
	}
	fixture.failFile = ""
	if err := store.DeleteSCIMProfile("target", `W/"2"`); err != nil {
		t.Fatal(err)
	}
	if !fixture.profiles[0].Disabled || !fixture.profiles[0].SCIMDeleted || fixture.profiles[0].Revision != 3 || len(fixture.sessions) != 0 {
		t.Fatalf("delete did not commit: %#v", fixture)
	}
}
