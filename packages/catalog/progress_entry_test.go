package catalog

import (
	"errors"
	"sync"
	"testing"
)

func TestRecordPersistencePublishesOnlyAfterCommit(t *testing.T) { //nolint:cyclop // The failure and success assertions share one durable commit sequence.
	values := map[string]PlaybackState{"viewer:item": {Seconds: 10}}
	var mu, persistMu sync.Mutex
	blocked := errors.New("disk unavailable")
	storage := ProgressStorage{
		Mutex: &mu, PersistMutex: &persistMu, Values: &values, File: "progress.json",
		Persist: func(string, any) error { t.Fatal("record update rewrote document"); return nil },
		PersistEntry: func(key string, value PlaybackState) error {
			if key != "viewer:item" || value.Seconds != 20 || values[key].Seconds != 10 {
				t.Fatal("state published before durable commit")
			}
			return blocked
		},
	}
	change := func(value PlaybackState) (PlaybackState, bool, error) { value.Seconds = 20; return value, true, nil }
	if _, _, changed, err := storage.Update("viewer:item", change); !errors.Is(err, blocked) || changed || values["viewer:item"].Seconds != 10 {
		t.Fatalf("failed commit changed memory: %v", err)
	}
	storage.PersistEntry = func(string, PlaybackState) error { return nil }
	if _, _, changed, err := storage.Update("viewer:item", change); err != nil || !changed || values["viewer:item"].Seconds != 20 {
		t.Fatalf("successful commit = %v", err)
	}
	storage.PersistEntry = func(string, PlaybackState) error { t.Fatal("invalid progress persisted"); return nil }
	if _, _, _, err := storage.Update("", change); !errors.Is(err, ErrInvalidProgressState) {
		t.Fatalf("invalid key = %v", err)
	}
}
