package catalog

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScanAndIndexHelpers(t *testing.T) { //nolint:cyclop // One test covers the coupled scan and publication pipeline.
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Movie (2024).mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := Scan(context.Background(), []ScanRoot{{directory, "Movies"}}, func(items []library.Item) []library.Item {
		items[0].Title = "Decorated"
		return items
	})
	if err != nil || len(items) != 1 || items[0].Title != "Decorated" || ItemsByID(append(items, items...))[items[0].ID] != 0 || ScanRoots([]string{directory}, func(path string) (string, string) { return path, "Movies" })[0].Path != directory {
		t.Fatalf("items = %#v, error = %v", items, err)
	}

	var mutex sync.RWMutex
	indexed, byID := []library.Item{{ID: "old", Title: "Old"}}, map[string]int{"old": 0}
	var scanErr error
	ready := false
	var scanned time.Time
	var decorator func([]library.Item) []library.Item
	var analyzers []func([]library.Item)
	storage := IndexStorage{Mutex: &mutex, Items: &indexed, ByID: &byID, Err: &scanErr, Ready: &ready, Scanned: &scanned, Decorator: &decorator, Analyzers: &analyzers}
	storage.SetDecorator(func(items []library.Item) []library.Item { items[0].Title += "!"; return items })
	analyzed, analysisRuns := "", 0
	storage.AddAnalyzer(func(items []library.Item) {
		analysisRuns++
		analyzed = items[0].Title
		if analysisRuns == 1 {
			items[0].Title = "detached"
		}
	})
	if analyzed != "Old!" || indexed[0].Title != "Old!" {
		t.Fatal("decorator or detached analyzer snapshot failed")
	}
	if err := storage.Refresh(context.Background(), []ScanRoot{{directory, "Movies"}}); err != nil {
		t.Fatal(err)
	}
	count, refreshedAt, statusErr := storage.Status()
	if count != 1 || refreshedAt.IsZero() || statusErr != nil || !ready || indexed[0].Title != "Movie!" {
		t.Fatalf("status = %d %v %v, ready=%t, items=%#v", count, refreshedAt, statusErr, ready, indexed)
	}
}

func TestIndexOwnsScanLifecycle(t *testing.T) { //nolint:cyclop,funlen // One test covers the coupled index lifecycle and its detached read boundaries.
	directory := t.TempDir()
	media := filepath.Join(directory, "Movie (2024).mp4")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	acquires := 0
	index := NewIndex(t.Context(), []ScanRoot{{Path: directory, Namespace: "Movies"}}, directory, time.Hour, func(context.Context) (func(), error) {
		acquires++
		return func() { acquires++ }, nil
	})
	items, err := index.Snapshot()
	if err != nil || len(items) != 1 || acquires != 2 || !index.Safe(media) || index.Safe(filepath.Join(t.TempDir(), "outside")) {
		t.Fatalf("initial index = %#v, error=%v, acquires=%d", items, err, acquires)
	}
	roots := index.Roots()
	roots[0].Path = "changed"
	if index.Roots()[0].Path != directory {
		t.Fatal("Roots exposed index storage")
	}

	index.SetDecorator(func(items []library.Item) []library.Item {
		items[0].Title = "Decorated"
		return items
	})
	analyzed := make(chan string, 3)
	index.AddAnalyzer(func(items []library.Item) { analyzed <- items[0].Title })
	if got := <-analyzed; got != "Decorated" {
		t.Fatalf("initial analysis = %q", got)
	}
	if err := index.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := <-analyzed; got != "Decorated" {
		t.Fatalf("refresh analysis = %q", got)
	}
	count, scanned, statusErr := index.Status()
	item, found := index.Find(items[0].ID)
	references, referenceErr := index.References()
	if count != 1 || scanned.IsZero() || statusErr != nil || !found || item.Title != "Decorated" || referenceErr != nil || len(references) != 1 {
		t.Fatalf("status=%d/%v/%v item=%#v found=%t references=%#v/%v", count, scanned, statusErr, item, found, references, referenceErr)
	}
	if _, found := index.Find("missing"); found {
		t.Fatal("missing item was found")
	}

	index.SetFrequency("5m")
	index.SetFrequency("default")
	index.SetFrequency("off")
	ctx, cancel := context.WithCancel(t.Context())
	index.Schedule(ctx)
	index.RequestRefresh()
	if got := <-analyzed; got != "Decorated" {
		t.Fatalf("scheduled analysis = %q", got)
	}
	cancel()

	memory := NewMemoryIndex([]library.Item{{ID: "memory", Title: "Memory"}}, true)
	memory.SetRoots([]ScanRoot{{Path: directory, Namespace: "Memory"}})
	memoryItems, memoryErr := memory.Snapshot()
	if memoryErr != nil || len(memoryItems) != 1 || memoryItems[0].ID != "memory" || memory.Roots()[0].Namespace != "Memory" {
		t.Fatalf("memory index = %#v, error=%v", memoryItems, memoryErr)
	}
	empty := NewIndex(t.Context(), nil, "", 0, nil)
	if emptyItems, emptyErr := empty.Snapshot(); emptyErr != nil || len(emptyItems) != 0 {
		t.Fatalf("empty index = %#v, error=%v", emptyItems, emptyErr)
	}
}

func TestIndexWatchRefreshesStableFilesystemChanges(t *testing.T) { //nolint:cyclop,gocognit // Watch registration and refresh publication form one lifecycle.
	directory := t.TempDir()
	first := filepath.Join(directory, "First.mp4")
	if err := os.WriteFile(first, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndex(t.Context(), []ScanRoot{{Path: directory, Namespace: "Movies"}}, "", time.Hour, nil)
	index.watchDebounce = 5 * time.Millisecond
	index.watchStability = 5 * time.Millisecond
	index.watchRetry = 5 * time.Millisecond
	index.pollInterval = 5 * time.Millisecond
	counts := make(chan int, 32)
	index.AddAnalyzer(func(items []library.Item) { counts <- len(items) })
	<-counts
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	index.Schedule(ctx)
	index.Watch(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for {
		watching, watchErr := index.Monitoring()
		if watchErr != nil {
			t.Fatal(watchErr)
		}
		if watching {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("filesystem watcher did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if roots := index.rootSet(); len(roots) != 1 {
		t.Fatalf("root set = %#v", roots)
	}
	second := filepath.Join(directory, "Second.mp4")
	if err := os.WriteFile(second, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case count := <-counts:
			if count == 2 {
				state, err := snapshotFiles(map[string]struct{}{directory: {}})
				if err != nil || len(state) != 2 {
					t.Fatalf("file snapshot = %#v, error=%v", state, err)
				}
				if _, err := snapshotFiles(map[string]struct{}{string([]byte{0}): {}}); err == nil {
					t.Fatal("invalid snapshot root was accepted")
				}
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("filesystem change was not refreshed")
		}
	}
}
