package markers

import (
	"context"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMarkerAnalyzerDetectsRecurringMovieStudioOpening(t *testing.T) {
	sharedOpening := sequence(200, 9000)
	items := movieItems(3)
	analyzer := NewAnalyzer(Config{})
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
	analyzer.extract = func(_ context.Context, item library.Item, window string, _, _ float64) ([]uint32, error) {
		if window != "opening" {
			return sequence(300, uint32(item.Size)*100000), nil //nolint:gosec // Fixture sizes are bounded.
		}
		return append(sequence(40+int(item.Size), uint32(item.Size)*100000), sharedOpening...), nil //nolint:gosec // Fixture sizes are bounded.
	}
	sharedVisual := visualSequence(45, 9000)
	analyzer.extractVisual = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint64, error) {
		return append(visualSequence(6+int(item.Size), uint64(item.Size)*100000), sharedVisual...), nil //nolint:gosec // Fixture sizes are bounded.
	}
	if err := analyzer.analyze(t.Context(), items); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		markers := analyzer.Markers(item, nil)
		if len(markers) != 1 || markers[0].Type != "intro" || markers[0].Source != "fingerprint" || markers[0].Start < 5 || markers[0].End-markers[0].Start < 20 {
			t.Fatalf("%s markers = %#v", item.ID, markers)
		}
	}
}

func TestMovieOpeningRequiresThreeTitlesAndVisualConsensus(t *testing.T) {
	sharedOpening := sequence(200, 9000)
	t.Run("two titles", func(t *testing.T) {
		items := movieItems(2)
		analyzer := NewAnalyzer(Config{})
		analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
		analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
		analyzer.extract = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint32, error) {
			return append(sequence(40+int(item.Size), uint32(item.Size)*100000), sharedOpening...), nil //nolint:gosec // Fixture sizes are bounded.
		}
		if err := analyzer.analyze(t.Context(), items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if markers := analyzer.Markers(item, nil); len(markers) != 0 {
				t.Fatalf("%s markers = %#v", item.ID, markers)
			}
		}
	})
	t.Run("audio only", func(t *testing.T) {
		items := movieItems(3)
		analyzer := NewAnalyzer(Config{})
		analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
		analyzer.extract = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint32, error) {
			return append(sequence(40+int(item.Size), uint32(item.Size)*100000), sharedOpening...), nil //nolint:gosec // Fixture sizes are bounded.
		}
		analyzer.extractVisual = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint64, error) {
			return visualSequence(300, uint64(item.Size)*100000), nil //nolint:gosec // Fixture sizes are bounded.
		}
		if err := analyzer.analyzeMovieOpenings(t.Context(), items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			markers := analyzer.Markers(item, nil)
			if len(markers) != 1 || markers[0].Source != "recurrence" {
				t.Fatalf("%s markers = %#v", item.ID, markers)
			}
		}
	})
}

func TestMarkerCandidatePairsAndStaleDetectorVersion(t *testing.T) {
	items := make([]episodeFingerprint, 1000)
	for index := range items {
		items[index].opening = sequence(300, uint32(index+1)*1000000) //nolint:gosec // Fixture index is bounded.
		items[index].openingVisual = visualSequence(300, uint64(index+1)*1000000)
	}
	if pairs := markerCandidatePairs(items); len(pairs) != 0 {
		t.Fatalf("unrelated candidate pairs = %d", len(pairs))
	}
	libraryItems := append(movieItems(2), library.Item{ID: "other", Kind: "video", Library: "Other", Size: 3, Added: time.Unix(3, 0)})
	analyzer := NewAnalyzer(Config{})
	for _, item := range libraryItems {
		analyzer.records[item.ID] = Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion}
	}
	stale := analyzer.records[libraryItems[0].ID]
	stale.DetectorVersion--
	analyzer.records[libraryItems[0].ID] = stale
	changed := analyzer.changedItems(libraryItems)
	if len(changed) != 2 || changed[0].ID != "one" || changed[1].ID != "two" {
		t.Fatalf("changed items = %#v", changed)
	}
}

func movieItems(count int) []library.Item {
	items := make([]library.Item, count)
	for index := range items {
		items[index] = library.Item{ID: []string{"one", "two", "three"}[index], Kind: "video", Library: "Movies", Size: int64(index + 1), Added: time.Unix(int64(index+1), 0)}
	}
	return items
}
