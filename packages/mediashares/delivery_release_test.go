package mediashares

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestStreamSlotIsReleasedAfterItsSessionIsPruned(t *testing.T) {
	started := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	clock := started
	state := new(memoryState)
	store := New(t.TempDir(), Dependencies{
		Load: state.load, Persist: state.persist,
		Find:     func(id string) (library.Item, bool) { return library.Item{ID: id, Path: "movie"}, id == "item" },
		Snapshot: func() ([]library.Item, error) { return []library.Item{{ID: "item", Path: "movie"}}, nil },
		Now:      func() time.Time { return clock }, Random: rand.Reader,
	})
	_, claim, err := store.Create([]string{"item"}, 24*time.Hour, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := store.Claim(claim, "First browser")
	if err != nil {
		t.Fatal(err)
	}
	_, release, ok := store.Item(capabilityRequest(t, first), "item")
	if !ok {
		t.Fatal("first device stream was rejected")
	}
	clock = started.Add(9 * time.Hour)
	second, _, err := store.Claim(claim, "Second browser")
	if err != nil {
		t.Fatal(err)
	}
	release()
	secondRequest := capabilityRequest(t, second)
	for stream := range 2 {
		if _, _, ok := store.Item(secondRequest, "item"); !ok {
			t.Fatalf("stream %d was rejected by a slot leaked from a pruned session", stream+1)
		}
	}
}
