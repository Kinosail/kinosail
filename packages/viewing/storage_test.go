package viewing

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestSyncStorageRejectsInvalidStateAndClonesSecrets(t *testing.T) { //nolint:cyclop,gocognit // One trust-boundary test covers strict restore and detached runtime updates.
	file := filepath.Join(t.TempDir(), "viewing_imports.json")
	normalize := func(input Input) (Input, error) {
		if input.ProfileID != "viewer" || input.Token == "" {
			return Input{}, errors.New("invalid input")
		}
		input.Source = strings.ToLower(input.Source)
		return input, nil
	}
	if syncs, err := LoadSyncs(file, normalize); err != nil || len(syncs) != 0 {
		t.Fatalf("missing state = %#v, error=%v", syncs, err)
	}
	valid := `{"ABCDEFGHIJKLMNOP234567":{"id":"ABCDEFGHIJKLMNOP234567","source":"PLEX","url":"https://plex.example","token":"secret","profileId":"viewer","interval":"1h","lastResult":{},"seen":{"movie":{"targetId":"target","signature":"sig"}}}}`
	if err := os.WriteFile(file, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	syncs, err := LoadSyncs(file, normalize)
	if err != nil || syncs["ABCDEFGHIJKLMNOP234567"].Source != "plex" {
		t.Fatalf("syncs=%#v error=%v", syncs, err)
	}
	cloned := CloneSyncs(syncs)
	cloned["ABCDEFGHIJKLMNOP234567"].Seen["movie"] = Observation{TargetID: "changed"}
	if syncs["ABCDEFGHIJKLMNOP234567"].Seen["movie"].TargetID != "target" || !ValidSyncInterval("15m") || ValidSyncInterval("weekly") {
		t.Fatal("sync clone or interval validation failed")
	}
	for _, invalid := range []string{
		`{`,
		strings.Replace(valid, `"lastResult":{}`, `"lastResult":{},"unknown":true`, 1),
		strings.Replace(valid, `"interval":"1h"`, `"interval":"weekly"`, 1),
		strings.Replace(valid, `"targetId":"target"`, `"targetId":""`, 1),
		strings.Replace(valid, `"lastResult":{}`, `"lastResult":{"applied":-1}`, 1),
	} {
		if err := os.WriteFile(file, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if restored, err := LoadSyncs(file, normalize); err == nil || restored != nil {
			t.Fatalf("invalid state restored: %#v", restored)
		}
	}
	if ValidPending(&SyncPending{Changes: []ProgressChange{{TargetID: ""}}}) || !ValidPending(nil) || ValidSummary(Summary{Applied: -1}) {
		t.Fatal("pending or summary validation accepted invalid state")
	}
	message := SafeError(errors.New(strings.Repeat("x", 2048) + "\nsecret"))
	if len(message) != 1024 || strings.Contains(message, "\n") || SafeError(nil) != "" {
		t.Fatalf("safe error = %q", message)
	}
}

func TestSyncStorageRejectsEveryPendingBound(t *testing.T) { //nolint:cyclop // Each persisted batch field is an independent trust boundary.
	largeChanges := make([]ProgressChange, maximumSyncItems+1)
	largeLists := make([]ListChange, maximumSyncItems+1)
	largePlaylists := make(map[string]int, maximumSyncPlaylists+1)
	for index := range maximumSyncPlaylists + 1 {
		largePlaylists[fmt.Sprintf("List %d", index)] = index
	}
	cases := []*SyncPending{
		{Changes: largeChanges},
		{ListChanges: largeLists},
		{Summary: Summary{Applied: -1}},
		{Changes: []ProgressChange{{TargetID: ""}}},
		{Changes: []ProgressChange{{TargetID: strings.Repeat("a", 513)}}},
		{Changes: []ProgressChange{{TargetID: "id", State: catalog.PlaybackState{Seconds: math.NaN()}}}},
		{Changes: []ProgressChange{{TargetID: "id", Expected: catalog.PlaybackState{Session: strings.Repeat("a", 513)}}}},
		{ListChanges: []ListChange{{TargetID: ""}}},
		{ListChanges: []ListChange{{TargetID: strings.Repeat("a", 513)}}},
		{ListChanges: []ListChange{{TargetID: "id", Playlists: largePlaylists}}},
		{ListChanges: []ListChange{{TargetID: "id", Playlists: map[string]int{"bad/name": 0}}}},
		{ListChanges: []ListChange{{TargetID: "id", Playlists: map[string]int{"Queue": -1}}}},
		{ListChanges: []ListChange{{TargetID: "id", Playlists: map[string]int{"Queue": maximumSyncItems}}}},
	}
	for index, pending := range cases {
		if ValidPending(pending) {
			t.Errorf("invalid pending %d accepted", index)
		}
	}
	manyMemberships := &SyncPending{ListChanges: make([]ListChange, 101)}
	for index := range manyMemberships.ListChanges {
		playlists := make(map[string]int, maximumSyncPlaylists)
		for list := range maximumSyncPlaylists {
			playlists[fmt.Sprintf("List %d", list)] = list
		}
		manyMemberships.ListChanges[index] = ListChange{TargetID: "id", Playlists: playlists}
	}
	if ValidPending(manyMemberships) {
		t.Fatal("membership aggregate accepted")
	}
	if !ValidPending(&SyncPending{Changes: []ProgressChange{{TargetID: "id"}}, ListChanges: []ListChange{{TargetID: "id", Playlists: map[string]int{"Queue": 0}}}}) {
		t.Fatal("valid pending rejected")
	}
}

func TestLoadSyncsRejectsOpenAndSizeFailures(t *testing.T) {
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	if state, err := LoadSyncs(loop, func(input Input) (Input, error) { return input, nil }); err == nil || state != nil {
		t.Fatalf("symlink state=%#v error=%v", state, err)
	}
	if state, err := LoadSyncs(t.TempDir(), func(input Input) (Input, error) { return input, nil }); err == nil || state != nil {
		t.Fatalf("directory state=%#v error=%v", state, err)
	}
	file := filepath.Join(t.TempDir(), "viewing_imports.json")
	if err := os.WriteFile(file, make([]byte, MaximumSyncStateSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if state, err := LoadSyncs(file, func(input Input) (Input, error) { return input, nil }); err == nil || state != nil {
		t.Fatalf("large state=%#v error=%v", state, err)
	}
}
