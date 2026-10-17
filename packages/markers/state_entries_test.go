package markers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

func markerStore(t *testing.T, directory string) func() *documentdb.Store {
	t.Helper()
	return func() *documentdb.Store {
		store, err := documentdb.Open(directory, false, documentdb.Config{Documents: []string{"playback_markers.json"}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		return store
	}
}

func TestDatabaseMarkersMigrateToEntriesAndSurviveRestart(t *testing.T) {
	directory := t.TempDir()
	open := markerStore(t, directory)
	items := movieItems(3)
	legacy := map[string]Record{items[0].ID: {Revision: mediaRevision(items[0]), DetectorVersion: DetectorVersion, Markers: []Marker{{Type: "intro", Label: "Intro", Start: 1, End: 9, Source: "manual"}}}}
	if err := open().SaveJSON("playback_markers.json", legacy); err != nil {
		t.Fatal(err)
	}
	analyzer := NewAnalyzer(Config{DataDir: directory, Database: open()})
	if analyzer.saveEntries == nil || len(analyzer.records) != 1 {
		t.Fatalf("legacy markers were not migrated: records=%d err=%v", len(analyzer.records), analyzer.err)
	}
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	if err := analyzer.SetManual(items[1], "credits", 7000, 7200, 7200); err != nil {
		t.Fatal(err)
	}
	if err := analyzer.analyzeBatch(t.Context(), items[2:], nil); err != nil {
		t.Fatal(err)
	}
	reloaded := NewAnalyzer(Config{DataDir: directory, Database: open()})
	if len(reloaded.records) != 3 || len(reloaded.Markers(items[0], nil)) != 1 || len(reloaded.Markers(items[1], nil)) != 1 || reloaded.records[items[2].ID].Revision != mediaRevision(items[2]) {
		t.Fatalf("reloaded markers = %#v", reloaded.records)
	}
}

func TestDatabaseMarkerBatchCommitsAllOrNothing(t *testing.T) {
	directory := t.TempDir()
	open := markerStore(t, directory)
	items := libraryMovies(40)
	items[len(items)-1].ID = strings.Repeat("r", 300)
	analyzer := NewAnalyzer(Config{DataDir: directory, Database: open()})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
	analyzer.extract = func(context.Context, library.Item, string, float64, float64) ([]uint32, error) {
		return nil, errors.New("audio unavailable")
	}
	analyzer.extractVisual = func(context.Context, library.Item, string, float64, float64) ([]uint64, error) {
		return nil, errors.New("visual unavailable")
	}
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	if err := analyzer.analyzeBatch(t.Context(), items, nil); !errors.Is(err, ErrPersistence) {
		t.Fatalf("rejected batch error = %v", err)
	}
	reloaded := NewAnalyzer(Config{DataDir: directory, Database: open()})
	if len(analyzer.records) != 0 || len(reloaded.records) != 0 {
		t.Fatalf("rejected batch left memory=%d disk=%d records", len(analyzer.records), len(reloaded.records))
	}
}
