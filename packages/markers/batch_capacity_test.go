package markers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestRunningMarkerBatchLeavesABackgroundSlotForOtherWork(t *testing.T) {
	governor := workload.New(4)
	items := make([]library.Item, 4)
	for index := range items {
		items[index] = library.Item{ID: fmt.Sprintf("movie-%d", index), Kind: "video", Library: fmt.Sprintf("Library-%d", index), Size: 1, Added: time.Unix(1, 0)}
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir(), BackgroundSlots: governor.Metrics().BackgroundCapacity, Acquire: func(ctx context.Context) (func(), error) {
		return governor.Acquire(ctx, workload.Background)
	}})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
	entered, release := make(chan string, len(items)), make(chan struct{})
	analyzer.extractCredits = func(ctx context.Context, item library.Item, _, _ float64) ([]byte, error) {
		entered <- item.ID
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		analyzer.work(ctx)
		close(done)
	}()
	analyzer.Enqueue(items)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("marker extraction did not start")
	}
	select {
	case <-entered:
	case <-time.After(250 * time.Millisecond):
	}
	other, stop := context.WithTimeout(t.Context(), 2*time.Second)
	defer stop()
	started := time.Now()
	releaseOther, err := governor.Acquire(other, workload.Background)
	if err != nil {
		close(release)
		t.Fatalf("background task waited %s behind the running marker batch: %v", time.Since(started).Round(time.Millisecond), err)
	}
	releaseOther()
	if state, _, _ := analyzer.Status(); state != "running" {
		t.Fatalf("marker batch state = %s while its extraction is blocked", state)
	}
	close(release)
	deadline := time.Now().Add(10 * time.Second)
	for state, _, _ := analyzer.Status(); state != "complete"; state, _, _ = analyzer.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("marker batch state = %s", state)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)}); len(reloaded.records) != len(items) || len(reloaded.changedItems(items)) != 0 {
		t.Fatalf("persisted records = %d", len(reloaded.records))
	}
}
