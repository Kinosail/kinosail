package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func writeMedia(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func eventually(t *testing.T, message string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func titles(index *catalog.Index) map[string]string {
	snapshot, _ := index.Library()
	result := make(map[string]string, snapshot.Len())
	for _, item := range snapshot.Items() {
		result[filepath.Base(item.Path)] = item.Title
	}
	return result
}

func TestStartupPublishesRawScanBeforeSlowEnrichmentFinishes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"One.flac", "Two.flac", "Three.flac", "Film (2020).mp4"} {
		writeMedia(t, filepath.Join(root, name))
	}
	release := make(chan struct{})
	analyzed := make(chan []library.Item, 8)
	decoration := catalog.Decoration{
		Apply: func(items []library.Item) []library.Item {
			for position := range items {
				items[position].Plot = "applied"
			}
			return items
		},
		Enriches: func(item library.Item) bool { return item.Kind == "audio" },
		Enrich: func(ctx context.Context, items []library.Item) []library.Item {
			select {
			case <-release:
			case <-ctx.Done():
			}
			for position := range items {
				items[position].Title = "Tagged " + items[position].Title
			}
			return items
		},
		Background: true,
	}
	started := time.Now()
	index := catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Music"}}, "", time.Hour, nil, decoration)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("startup waited for enrichment: %s", elapsed)
	}
	snapshot, err := index.Library()
	if err != nil || snapshot.Len() != 4 || titles(index)["One.flac"] != "One" || snapshot.Items()[0].Plot != "applied" {
		t.Fatalf("first publish = %#v, %v", snapshot.Items(), err)
	}
	index.AddAnalyzer(func(items []library.Item) { analyzed <- items })
	<-analyzed
	close(release)
	eventually(t, "enrichment was not republished", func() bool { return titles(index)["Three.flac"] == "Tagged Three" })
	if final := <-analyzed; len(final) != 4 || titles(index)["Film (2020).mp4"] != "Film" {
		t.Fatalf("analyzers saw %#v", final)
	}
	if err := index.Refresh(t.Context()); err != nil || titles(index)["One.flac"] != "Tagged One" {
		t.Fatalf("rescan lost cached enrichment: %#v, %v", titles(index), err)
	}
}

func TestRedecorateRepublishesWithoutWalkingTheLibrary(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, filepath.Join(root, "Film (2020).mp4"))
	var plot atomic.Value
	plot.Store("first")
	index := catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, nil, catalog.Decoration{Apply: func(items []library.Item) []library.Item {
		for position := range items {
			items[position].Plot = plot.Load().(string)
		}
		return items
	}})
	analyses := atomic.Int32{}
	index.AddAnalyzer(func([]library.Item) { analyses.Add(1) })
	writeMedia(t, filepath.Join(root, "Unscanned (2021).mp4"))
	before, _ := index.Library()
	index.Redecorate()
	if after, _ := index.Library(); after != before || analyses.Load() != 1 {
		t.Fatalf("unchanged redecorate republished: generation %d -> %d, analyses %d", before.Generation(), after.Generation(), analyses.Load())
	}
	plot.Store("second")
	index.Redecorate()
	after, _ := index.Library()
	if after.Len() != 1 || after.Items()[0].Plot != "second" || after.Generation() != before.Generation()+1 || analyses.Load() != 2 {
		t.Fatalf("redecorate = %#v generation %d analyses %d", after.Items(), after.Generation(), analyses.Load())
	}
}

func TestChangedPathsRescanOnlyTheirDirectories(t *testing.T) {
	root := t.TempDir()
	movies, shows := filepath.Join(root, "Movies"), filepath.Join(root, "Show", "Season 01")
	writeMedia(t, filepath.Join(movies, "Film (2020).mp4"), filepath.Join(shows, "Show S01E01.mkv"))
	index := catalog.NewIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Library"}}, "", time.Hour, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	index.Schedule(ctx)
	added, unannounced := filepath.Join(movies, "Sequel (2022).mp4"), filepath.Join(shows, "Show S01E02.mkv")
	writeMedia(t, added, unannounced, filepath.Join(shows, "Show S01E01.nfo"))
	index.RequestChanges([]string{added})
	eventually(t, "changed directory was not rescanned", func() bool { return titles(index)["Sequel (2022).mp4"] == "Sequel" })
	if names := titles(index); len(names) != 2+1 || names["Show S01E02.mkv"] != "" {
		t.Fatalf("incremental scan touched unannounced directories: %#v", names)
	}
	if err := os.Remove(filepath.Join(movies, "Film (2020).mp4")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shows, "Show S01E01.nfo"), []byte("<episodedetails><title>Pilot</title></episodedetails>"), 0o600); err != nil {
		t.Fatal(err)
	}
	index.RequestChanges([]string{filepath.Join(movies, "Film (2020).mp4"), filepath.Join(shows, "Show S01E01.nfo")})
	eventually(t, "removal and sidecar edit were not rescanned", func() bool {
		names := titles(index)
		return names["Film (2020).mp4"] == "" && names["Show S01E01.mkv"] == "S01E01 · Pilot" && names["Show S01E02.mkv"] != ""
	})
	if err := index.Refresh(t.Context()); err != nil || len(titles(index)) != 3 {
		t.Fatalf("full rescan = %#v, %v", titles(index), err)
	}
}

func TestWatcherStartupDoesNotRepeatTheInitialScan(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, filepath.Join(root, "Film (2020).mp4"))
	scans := atomic.Int32{}
	index := catalog.NewIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, func(context.Context) (func(), error) {
		scans.Add(1)
		return func() {}, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	index.Schedule(ctx)
	index.Watch(ctx)
	eventually(t, "watcher did not start", func() bool { watching, _ := index.Monitoring(); return watching })
	time.Sleep(200 * time.Millisecond)
	if scans.Load() != 1 {
		t.Fatalf("library scans after watcher startup = %d, want 1", scans.Load())
	}
}
