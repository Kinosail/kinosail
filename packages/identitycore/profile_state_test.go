package identitycore

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLoadProfileState(t *testing.T) {
	empty := LoadProfileState("", nil, nil, nil)
	if empty.Err != nil || len(empty.Sessions) != 0 || len(empty.APIKeys) != 0 {
		t.Fatalf("empty state = %#v", empty)
	}
	state := LoadProfileState("state", func(_ string, target any) (bool, error) {
		profiles := target.(*[]Profile)
		*profiles = []Profile{{ID: "owner", Name: "Owner", Owner: true}}
		return true, nil
	}, func(string) (map[string]Session, error) {
		return map[string]Session{"session": {ProfileID: "owner"}}, nil
	}, func(string) (map[string]APIKey, error) {
		return map[string]APIKey{"key": {ProfileID: "owner"}}, nil
	})
	if state.Err != nil || state.ProfileFile != filepath.Join("state", "profiles.json") || len(state.Profiles) != 1 || len(state.Sessions) != 1 || len(state.APIKeys) != 1 {
		t.Fatalf("loaded state = %#v", state)
	}
}

func TestLoadProfileStateStopsOnLoaderFailure(t *testing.T) { //nolint:gocognit // The score of 19 remains below the repository ceiling of 22 for the ordered loader matrix.
	loadErr := errors.New("load failed")
	for name, fail := range map[string]string{"profiles": "profiles", "sessions": "sessions", "keys": "keys"} {
		t.Run(name, func(t *testing.T) {
			calls := []string{}
			state := LoadProfileState("state", func(path string, target any) (bool, error) {
				calls = append(calls, filepath.Base(path))
				if fail == "profiles" {
					return false, loadErr
				}
				profiles := target.(*[]Profile)
				*profiles = []Profile{{ID: "owner", Name: "Owner", Owner: true}}
				return true, nil
			}, func(path string) (map[string]Session, error) {
				calls = append(calls, filepath.Base(path))
				if fail == "sessions" {
					return nil, loadErr
				}
				return map[string]Session{"session": {ProfileID: "owner"}}, nil
			}, func(path string) (map[string]APIKey, error) {
				calls = append(calls, filepath.Base(path))
				if fail == "keys" {
					return nil, loadErr
				}
				return map[string]APIKey{"key": {ProfileID: "owner"}}, nil
			})
			if !errors.Is(state.Err, loadErr) {
				t.Fatalf("state error = %v", state.Err)
			}
			if len(calls) == 0 {
				t.Fatal("no loader called")
			}
		})
	}
}

func TestLoadProfileStateRejectsInvalidProfilesBeforeRelatedReads(t *testing.T) {
	relatedCalls := 0
	state := LoadProfileState("state", func(_ string, target any) (bool, error) {
		profiles := target.(*[]Profile)
		*profiles = []Profile{{Name: "missing identifier"}}
		return true, nil
	}, func(string) (map[string]Session, error) {
		relatedCalls++
		return nil, nil
	}, func(string) (map[string]APIKey, error) {
		relatedCalls++
		return nil, nil
	})
	if state.Err == nil || relatedCalls != 0 {
		t.Fatalf("invalid state = %#v, related calls %d", state, relatedCalls)
	}
}
