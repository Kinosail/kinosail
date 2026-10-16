package catalog

import (
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

func coalescingProgressStore(t *testing.T) (*RequestProgressStore, func() PlaybackState, *documentdb.Store) {
	t.Helper()
	directory := t.TempDir()
	database, err := documentdb.Open(directory, true, documentdb.Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	viewer := func(*http.Request) ProgressViewer { return ProgressViewer{ID: "viewer"} }
	store := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, viewer, nil)
	if store.Err() != nil {
		t.Fatal(store.Err())
	}
	persisted := func() PlaybackState {
		restored := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, viewer, nil)
		if restored.Err() != nil {
			t.Fatal(restored.Err())
		}
		return restored.GetFor("viewer", false, "movie")
	}
	return store, persisted, database
}

func TestProgressHeartbeatsCoalesceUntilFlushOrStop(t *testing.T) {
	store, persisted, _ := coalescingProgressStore(t)
	stop := store.StartWriteCoalescing(time.Hour)
	request := progressRequest("viewer", false)
	if err := store.Set(request, "movie", 10, nil); err != nil || persisted().Seconds != 10 {
		t.Fatalf("first position = %v, %v", persisted(), err)
	}
	for _, seconds := range []float64{20, 30} {
		if err := store.Set(request, "movie", seconds, nil); err != nil {
			t.Fatal(err)
		}
	}
	if store.Get(request, "movie").Seconds != 30 || persisted().Seconds != 10 {
		t.Fatalf("heartbeat memory=%v persisted=%v", store.Get(request, "movie"), persisted())
	}
	store.Stop(request, "movie", 30)
	if persisted().Seconds != 30 {
		t.Fatalf("stop did not flush: %v", persisted())
	}
	if err := store.Set(request, "movie", 5, nil); err != nil || persisted().Seconds != 5 {
		t.Fatalf("position reset = %v, %v", persisted(), err)
	}
	watched := true
	if err := store.Set(request, "movie", 6, &watched); err != nil || !persisted().Watched {
		t.Fatalf("watched = %v, %v", persisted(), err)
	}
	if err := store.Set(request, "movie", 7, nil); err != nil || persisted().Seconds != 7 || persisted().Watched {
		t.Fatalf("resume after watched = %v, %v", persisted(), err)
	}
	if err := store.Set(request, "movie", 40, nil); err != nil || persisted().Seconds != 7 {
		t.Fatalf("heartbeat = %v, %v", persisted(), err)
	}
	stop()
	if persisted().Seconds != 40 {
		t.Fatalf("shutdown did not flush: %v", persisted())
	}
	if err := store.Set(request, "movie", 50, nil); err != nil || persisted().Seconds != 50 {
		t.Fatalf("write after shutdown = %v, %v", persisted(), err)
	}
}

func TestProgressFlushFailureMakesLaterHeartbeatsSynchronous(t *testing.T) {
	store, _, database := coalescingProgressStore(t)
	stop := store.StartWriteCoalescing(time.Hour)
	defer stop()
	request := progressRequest("viewer", false)
	if err := store.Set(request, "movie", 10, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(request, "movie", 20, nil); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Flush(); err == nil {
		t.Fatal("flush to a closed database succeeded")
	}
	if err := store.Set(request, "movie", 30, nil); err == nil {
		t.Fatal("heartbeat after a failed flush was accepted without storage")
	}
	if got := store.Get(request, "movie").Seconds; got != 20 {
		t.Fatalf("failed heartbeat changed memory to %v", got)
	}
}
