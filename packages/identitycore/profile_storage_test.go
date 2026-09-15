package identitycore

import (
	"errors"
	"reflect"
	"testing"
)

type profileBatchStub struct {
	values map[string]any
	err    error
}

func (stub *profileBatchStub) SaveJSONBatch(values map[string]any) error {
	stub.values = values
	return stub.err
}

func TestNewProfilePersistenceKeepsDisabledDatabaseNil(t *testing.T) {
	t.Parallel()
	var database *profileBatchStub
	persistence := NewProfilePersistence(database, false, nil, "profiles", "sessions", "keys")
	if persistence.Database != nil || persistence.ProfileFile != "profiles" || persistence.SessionFile != "sessions" || persistence.APIFile != "keys" {
		t.Fatalf("NewProfilePersistence(disabled) = %#v", persistence)
	}
	database = &profileBatchStub{}
	persistence = NewProfilePersistence(database, true, nil, "profiles", "sessions", "keys")
	if persistence.Database != database {
		t.Fatalf("NewProfilePersistence(enabled) = %#v", persistence)
	}
}

func TestProfilePersistenceSave(t *testing.T) {
	t.Parallel()
	profiles := []Profile{{ID: "viewer"}}
	for name, persistence := range map[string]ProfilePersistence{
		"missing file":   {Persist: func(string, any) error { t.Fatal("unexpected write"); return nil }},
		"missing writer": {ProfileFile: "profiles.json"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := persistence.Save(profiles); err == nil {
				t.Fatal("Save accepted incomplete storage")
			}
		})
	}
	wantErr := errors.New("write failed")
	persistence := ProfilePersistence{ProfileFile: "profiles.json", Persist: func(path string, value any) error {
		if path != "profiles.json" || !reflect.DeepEqual(value, profiles) {
			t.Fatalf("Persist(%q, %#v)", path, value)
		}
		return wantErr
	}}
	if err := persistence.Save(profiles); !errors.Is(err, wantErr) {
		t.Fatalf("Save(error) = %v", err)
	}
	persistence.Persist = func(string, any) error { return nil }
	if err := persistence.Save(profiles); err != nil {
		t.Fatalf("Save(success) = %v", err)
	}
}

func TestProfilePersistenceSaveRelatedDatabase(t *testing.T) {
	t.Parallel()
	profiles := []Profile{{ID: "viewer"}}
	sessions := map[string]Session{"session": {ProfileID: "viewer"}}
	keys := map[string]APIKey{"key": {ProfileID: "viewer"}}
	stub := &profileBatchStub{}
	persistence := ProfilePersistence{Database: stub, Persist: func(string, any) error {
		t.Fatal("file persistence used with database")
		return nil
	}}
	if err := persistence.SaveRelated(profiles, sessions, keys); err != nil {
		t.Fatalf("SaveRelated(database) = %v", err)
	}
	want := map[string]any{"profiles.json": profiles, "sessions.json": sessions, "api_keys.json": keys}
	if !reflect.DeepEqual(stub.values, want) {
		t.Fatalf("SaveJSONBatch() = %#v; want %#v", stub.values, want)
	}
	wantErr := errors.New("batch failed")
	stub.err = wantErr
	if err := persistence.SaveRelated(profiles, sessions, keys); !errors.Is(err, wantErr) {
		t.Fatalf("SaveRelated(database error) = %v", err)
	}
}

func TestProfilePersistenceRejectsIncompleteFiles(t *testing.T) {
	t.Parallel()
	profiles := []Profile{{ID: "viewer"}}
	sessions := map[string]Session{"session": {ProfileID: "viewer"}}
	keys := map[string]APIKey{"key": {ProfileID: "viewer"}}
	if err := (ProfilePersistence{}).SaveRelated(profiles, sessions, keys); err == nil {
		t.Fatal("SaveRelated accepted incomplete storage")
	}
	for name, persistence := range map[string]ProfilePersistence{
		"missing session file": {ProfileFile: "profiles.json", APIFile: "api_keys.json", Persist: func(string, any) error { t.Fatal("unexpected write"); return nil }},
		"missing API file":     {ProfileFile: "profiles.json", SessionFile: "sessions.json", Persist: func(string, any) error { t.Fatal("unexpected write"); return nil }},
	} {
		t.Run(name, func(t *testing.T) {
			if err := persistence.SaveRelated(profiles, sessions, keys); err == nil {
				t.Fatal("SaveRelated accepted incomplete paths")
			}
		})
	}
}

func TestProfilePersistenceSaveRelatedFiles(t *testing.T) { //nolint:gocognit // The score of 17 remains below the repository ceiling of 22 for the atomic write matrix.
	t.Parallel()
	profiles := []Profile{{ID: "viewer"}}
	sessions := map[string]Session{"session": {ProfileID: "viewer"}}
	keys := map[string]APIKey{"key": {ProfileID: "viewer"}}
	wantErr := errors.New("write failed")
	for failAt := 0; failAt <= 3; failAt++ {
		failAt := failAt
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			calls := make([]string, 0, 3)
			persistence := ProfilePersistence{
				ProfileFile: "profiles.json", SessionFile: "sessions.json", APIFile: "api_keys.json",
				Persist: func(path string, _ any) error {
					calls = append(calls, path)
					if len(calls) == failAt {
						return wantErr
					}
					return nil
				},
			}
			err := persistence.SaveRelated(profiles, sessions, keys)
			if failAt == 0 {
				if err != nil || !reflect.DeepEqual(calls, []string{"sessions.json", "api_keys.json", "profiles.json"}) {
					t.Fatalf("SaveRelated(success) = %v, %v", err, calls)
				}
				return
			}
			if !errors.Is(err, wantErr) || len(calls) != failAt {
				t.Fatalf("SaveRelated(fail %d) = %v, %v", failAt, err, calls)
			}
		})
	}
}
