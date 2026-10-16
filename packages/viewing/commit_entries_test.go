package viewing

import (
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestCommitStoreWritesChangedEntriesWithListsInOneTransaction(t *testing.T) {
	store, database := entryCommitStore(t, 3)
	updated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	changes := []ProgressChange{
		{TargetID: "item-0", Expected: catalog.PlaybackState{Seconds: 1, Updated: time.Unix(0, 0).UTC()}, State: catalog.PlaybackState{Seconds: 50, Updated: updated}},
		{TargetID: "item-1", Expected: catalog.PlaybackState{Seconds: 99}, State: catalog.PlaybackState{Seconds: 60, Updated: updated}},
		{TargetID: "item-7", State: catalog.PlaybackState{Seconds: 70, Updated: updated}},
	}
	applied, conflicts, listsApplied, err := store.Commit(Profile{ID: "viewer"}, changes, []ListChange{{TargetID: "item-9", Favorite: true}})
	if err != nil || applied != 2 || conflicts != 1 || listsApplied != 1 {
		t.Fatalf("commit = %d/%d/%d, %v", applied, conflicts, listsApplied, err)
	}
	want := map[string]float64{"viewer:item-0": 50, "viewer:item-1": 2, "viewer:item-2": 3, "viewer:item-7": 70}
	var persisted map[string]catalog.PlaybackState
	if _, err := database.LoadJSON("progress.json", &persisted); err != nil || len(persisted) != len(want) {
		t.Fatalf("persisted progress = %#v, %v", persisted, err)
	}
	for key, seconds := range want {
		if persisted[key].Seconds != seconds || (*store.Progress.Values)[key].Seconds != seconds {
			t.Fatalf("%s persisted=%v memory=%v want %v", key, persisted[key], (*store.Progress.Values)[key], seconds)
		}
	}
	var listed map[string]bool
	if _, err := database.LoadJSON("lists.json", &listed); err != nil || !listed["viewer:item-9"] || !(*store.Lists.Values)["viewer:item-9"] {
		t.Fatalf("persisted lists = %#v, %v", listed, err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	failed := []ProgressChange{{TargetID: "item-2", Expected: catalog.PlaybackState{Seconds: 3, Updated: time.Unix(2, 0).UTC()}, State: catalog.PlaybackState{Seconds: 80, Updated: updated}}}
	if _, _, _, err := store.Commit(Profile{ID: "viewer"}, failed, []ListChange{{TargetID: "item-8", Favorite: true}}); err == nil {
		t.Fatal("commit to a closed database succeeded")
	}
	if (*store.Progress.Values)["viewer:item-2"].Seconds != 3 || (*store.Lists.Values)["viewer:item-8"] {
		t.Fatal("failed commit changed memory")
	}
}

func TestEntryProgressImportWritesOnlyAcceptedChanges(t *testing.T) {
	commit, database := entryCommitStore(t, 2)
	storage := commit.Progress
	storage.ProfileID = "viewer"
	updated := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	applied, conflicts, err := storage.Import([]ProgressChange{
		{TargetID: "item-1", Expected: catalog.PlaybackState{Seconds: 2, Updated: time.Unix(1, 0).UTC()}, State: catalog.PlaybackState{Seconds: 20, Updated: updated}},
		{TargetID: "item-0", Expected: catalog.PlaybackState{Seconds: 7}, State: catalog.PlaybackState{Seconds: 30, Updated: updated}},
	}, time.Now())
	if err != nil || applied != 1 || conflicts != 1 {
		t.Fatalf("import = %d/%d, %v", applied, conflicts, err)
	}
	var persisted map[string]catalog.PlaybackState
	if _, err := database.LoadJSON("progress.json", &persisted); err != nil || persisted["viewer:item-1"].Seconds != 20 || persisted["viewer:item-0"].Seconds != 1 || len(persisted) != 2 {
		t.Fatalf("persisted = %#v, %v", persisted, err)
	}
	if (*storage.Values)["viewer:item-1"].Seconds != 20 {
		t.Fatalf("memory = %#v", *storage.Values)
	}
}
