package scimapp

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

type scimConfigurationEffects struct {
	current, managedKey, source, directory, token, expiration string
	setErr, deleteErr                                         error
	sets, deletes                                             int
	updates                                                   map[string]string
	deleted                                                   map[string]bool
}

func TestConfigurationStoreRejectsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	effects := &scimConfigurationEffects{}
	store := testSCIMConfigurationStore(effects)
	store.File = ""
	if err := store.Change("token", "expiration", false); err == nil || effects.sets != 0 {
		t.Fatalf("missing storage = %v %#v", err, effects)
	}
	store.File, effects.managedKey, effects.source = "/data/settings.json", TokenKey, "environment"
	if err := store.Change("token", "expiration", false); err == nil || !strings.Contains(err.Error(), "environment") || effects.sets != 0 {
		t.Fatalf("managed storage = %v %#v", err, effects)
	}
	effects.managedKey, effects.setErr = "", errors.New("invalid settings")
	if err := store.Change("token", "expiration", false); !errors.Is(err, effects.setErr) || effects.sets != 1 || len(effects.updates) != 0 {
		t.Fatalf("failed set = %v %#v", err, effects)
	}
}

func TestConfigurationStoreSetAndReset(t *testing.T) { //nolint:cyclop // One lifecycle verifies retained, explicit, reset, and failed values.
	t.Parallel()
	effects := &scimConfigurationEffects{current: "current-token"}
	store := testSCIMConfigurationStore(effects)
	if err := store.Change("", "expiration", false); err != nil {
		t.Fatal(err)
	}
	if effects.directory != "/data" || effects.token != "current-token" || effects.expiration != "expiration" || effects.updates[TokenKey] != "current-token" || effects.updates[ExpirationKey] != "expiration" {
		t.Fatalf("set effects = %#v", effects)
	}
	effects.updates, effects.deleted = map[string]string{}, map[string]bool{}
	if err := store.Change("explicit", "later", false); err != nil || effects.token != "explicit" {
		t.Fatalf("explicit set = %v %#v", err, effects)
	}
	if err := store.Change("", "", true); err != nil || effects.deletes != 1 || !effects.deleted[TokenKey] || !effects.deleted[ExpirationKey] {
		t.Fatalf("reset effects = %v %#v", err, effects)
	}
	effects.deleteErr = errors.New("delete failed")
	if err := store.Change("", "", true); !errors.Is(err, effects.deleteErr) || effects.deletes != 2 {
		t.Fatalf("failed reset = %v %#v", err, effects)
	}
}

func testSCIMConfigurationStore(effects *scimConfigurationEffects) ConfigurationStore[string] {
	effects.updates, effects.deleted = map[string]string{}, map[string]bool{}
	return ConfigurationStore[string]{
		File: "/data/settings.json", Lock: &sync.Mutex{}, Current: func(string) string { return effects.current },
		Managed: func(key string) bool { return key == effects.managedKey }, Source: func(string) string { return effects.source },
		Set: func(directory, token, expiration string) error {
			effects.sets++
			effects.directory, effects.token, effects.expiration = directory, token, expiration
			return effects.setErr
		},
		Delete: func(directory string) error {
			effects.deletes++
			effects.directory = directory
			return effects.deleteErr
		},
		Update: func(key, value string, deleted bool) {
			effects.updates[key], effects.deleted[key] = value, deleted
		},
	}
}
