package server_test

import (
	"encoding/json"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func storedState(t *testing.T, dataDir, name string) []byte {
	t.Helper()
	store, err := database.Open(dataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	data, found, err := store.Load(name)
	if err != nil || !found {
		t.Fatalf("load %s = %v, %v", name, found, err)
	}
	return data
}

func storedProfileID(t *testing.T, dataDir, name string) string {
	t.Helper()
	data := storedState(t, dataDir, "profiles.json")
	var profiles []struct{ ID, Name string }
	if err := json.Unmarshal(data, &profiles); err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.Name == name {
			return profile.ID
		}
	}
	t.Fatalf("profile %q was not found", name)
	return ""
}
