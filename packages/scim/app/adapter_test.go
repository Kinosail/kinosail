package scimapp

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRepositoryAdapterProjectsEveryStoreOperation(t *testing.T) { //nolint:cyclop // One repository lifecycle verifies every adapter method.
	t.Parallel()
	type record struct{ id string }
	wantErr := errors.New("store failed")
	created, updated, deleted := ProfileInput{}, ProfileInput{}, ""
	adapter := RepositoryAdapter[record]{
		ListProfiles: func() []record { return []record{{"one"}, {"two"}} },
		GetProfile: func(id string) (record, bool) {
			return record{id}, id != "missing"
		},
		CreateProfile: func(input ProfileInput) (record, error) {
			created = input
			return record{"created"}, wantErr
		},
		UpdateProfile: func(id string, input ProfileInput, expected string) (record, error) {
			updated = input
			return record{id + expected}, wantErr
		},
		DeleteProfile: func(id, expected string) error {
			deleted = id + expected
			return wantErr
		},
		Project: func(value record) Profile { return Profile{ID: value.id} },
	}
	listed := adapter.List()
	found, ok := adapter.Get("found")
	missing, missingOK := adapter.Get("missing")
	createInput, updateInput := ProfileInput{Name: "create"}, ProfileInput{Name: "update"}
	createdProfile, createErr := adapter.Create(createInput)
	updatedProfile, updateErr := adapter.Update("updated", updateInput, "-etag")
	deleteErr := adapter.Delete("deleted", "-etag")
	if len(listed) != 2 || listed[0].ID != "one" || listed[1].ID != "two" {
		t.Fatalf("list = %#v", listed)
	}
	if !ok || found.ID != "found" || missingOK || !reflect.DeepEqual(missing, Profile{}) {
		t.Fatalf("get = %#v %t, missing %#v %t", found, ok, missing, missingOK)
	}
	if !reflect.DeepEqual(created, createInput) || createdProfile.ID != "created" || !errors.Is(createErr, wantErr) {
		t.Fatalf("create = %#v %#v %v", created, createdProfile, createErr)
	}
	if !reflect.DeepEqual(updated, updateInput) || updatedProfile.ID != "updated-etag" || !errors.Is(updateErr, wantErr) {
		t.Fatalf("update = %#v %#v %v", updated, updatedProfile, updateErr)
	}
	if deleted != "deleted-etag" || !errors.Is(deleteErr, wantErr) {
		t.Fatalf("delete = %q %v", deleted, deleteErr)
	}
}

func TestNewRepositoryAndProfileProjection(t *testing.T) { //nolint:cyclop // One projection assertion verifies every protocol field.
	t.Parallel()
	type record struct{ id string }
	repository := NewRepository(
		func() []record { return []record{{"one"}} },
		func(id string) (record, bool) { return record{id}, true },
		func(ProfileInput) (record, error) { return record{"created"}, nil },
		func(string, ProfileInput, string) (record, error) { return record{"updated"}, nil },
		func(string, string) error { return nil },
		func(value record) Profile { return Profile{ID: value.id} },
	)
	if profiles := repository.List(); len(profiles) != 1 || profiles[0].ID != "one" {
		t.Fatalf("repository = %#v", profiles)
	}
	createdAt, updatedAt := time.Unix(1, 0), time.Unix(2, 0)
	emails := []Email{{Value: "viewer@example.com"}}
	profile := ProjectProfile("id", "name", "user", "external", Name{Formatted: "Name"}, emails, EnterpriseProfile{Department: "Media"}, true, createdAt, updatedAt, 9)
	emails[0].Value = "changed@example.com"
	if profile.ID != "id" || profile.Name != "name" || profile.UserName != "user" || profile.ExternalID != "external" || profile.NameParts.Formatted != "Name" || profile.Emails[0].Value != "viewer@example.com" || profile.Enterprise.Department != "Media" || !profile.Disabled || profile.CreatedAt != createdAt || profile.UpdatedAt != updatedAt || profile.Revision != 9 {
		t.Fatalf("profile = %#v", profile)
	}
}
