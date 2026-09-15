package markers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScheduleRunsChangedAnalysisAndReportsStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var changed func([]library.Item)
	analyzer := NewAnalyzer(Config{})
	analyzer.SetWait(func(context.Context) error { return nil })
	analyzer.Schedule(ctx, func(callback func([]library.Item)) { changed = callback }, func(context.Context, library.Item) Media { return Media{Duration: 50} })
	if changed == nil {
		t.Fatal("library observer was not registered")
	}
	item := library.Item{ID: "movie", Kind: "video", Library: "Movies", Size: 1, Added: time.Unix(1, 0)}
	changed([]library.Item{item})
	waitForMarkerState(t, analyzer, "complete")
	state, items, message := analyzer.Status()
	if state != "complete" || items != 1 || message != "" {
		t.Fatalf("status = %q, %d, %q", state, items, message)
	}
	changed([]library.Item{{Kind: "audio"}})
	waitForMarkerState(t, analyzer, "complete")
}

func TestScheduleReportsWaitAndAcquireFailures(t *testing.T) {
	for name, analyzer := range map[string]*Analyzer{
		"wait": NewAnalyzer(Config{}),
		"acquire": NewAnalyzer(Config{Acquire: func(context.Context) (func(), error) {
			return nil, errors.New("capacity unavailable")
		}}),
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if name == "wait" {
				analyzer.SetWait(func(context.Context) error { return errors.New("not idle") })
			}
			analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{} })
			go analyzer.work(ctx)
			analyzer.Enqueue([]library.Item{{ID: name, Kind: "video"}})
			waitForMarkerState(t, analyzer, "failed")
			_, _, message := analyzer.Status()
			if message == "" {
				t.Fatal("failure status omitted the error")
			}
		})
	}
}

func TestEnqueueKeepsOnlyTheNewestPendingBatch(t *testing.T) {
	analyzer := NewAnalyzer(Config{})
	analyzer.Enqueue([]library.Item{{ID: "old"}})
	analyzer.Enqueue([]library.Item{{ID: "new"}})
	batch := <-analyzer.jobs
	if len(batch) != 1 || batch[0].ID != "new" {
		t.Fatalf("pending batch = %#v", batch)
	}
}

func TestScheduleRejectsMissingAdapters(t *testing.T) {
	analyzer := NewAnalyzer(Config{})
	analyzer.Schedule(nil, func(func([]library.Item)) { t.Fatal("observer called") }, func(context.Context, library.Item) Media { return Media{} }) //nolint:staticcheck // Explicitly proves nil is rejected.
	analyzer.Schedule(t.Context(), nil, func(context.Context, library.Item) Media { return Media{} })
	analyzer.Schedule(t.Context(), func(func([]library.Item)) { t.Fatal("observer called") }, nil)
}

func waitForMarkerState(t *testing.T, analyzer *Analyzer, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, _, _ := analyzer.Status()
		if state == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	state, _, message := analyzer.Status()
	t.Fatalf("marker state = %q, want %q: %s", state, want, message)
}
