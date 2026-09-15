package markers

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFailedBatchPreservesPublishedMarkersAndRetries(t *testing.T) {
	for _, failure := range []string{"duration", "extract", "malformed", "cancel", "persist"} {
		t.Run(failure, func(t *testing.T) {
			items := movieItems(2)
			analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
			if err := analyzer.SetManual(items[0], "intro", 1, 10, 200); err != nil {
				t.Fatal(err)
			}
			before := maps.Clone(analyzer.records)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			analyzer.SetProbe(func(_ context.Context, item library.Item) Media {
				if failure == "duration" && item.ID == items[1].ID {
					return Media{}
				}
				return Media{Duration: 200}
			})
			analyzer.extractCredits = func(_ context.Context, item library.Item, _, _ float64) ([]byte, error) {
				if item.ID == items[1].ID {
					switch failure {
					case "extract":
						return nil, errors.New("private/media/path")
					case "malformed":
						return []byte{1}, nil
					case "cancel":
						cancel()
					}
				}
				return nil, nil
			}
			persist := analyzer.persist
			if failure == "persist" {
				analyzer.persist = func(string, any) error { return errors.New("private/storage/path") }
			}
			if err := analyzer.analyzeBatch(ctx, items); err == nil || strings.Contains(err.Error(), "private/") {
				t.Fatalf("failure = %v", err)
			}
			if !reflect.DeepEqual(analyzer.records, before) {
				t.Fatal("failed batch published partial results")
			}
			if got := analyzer.changedItems(items); len(got) != len(items) {
				t.Fatalf("retry items = %#v", got)
			}
			if reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)}); !reflect.DeepEqual(reloaded.records, before) {
				t.Fatal("failed batch changed durable results")
			}
			analyzer.persist = persist
			analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 200} })
			analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
			if err := analyzer.analyzeBatch(t.Context(), items); err != nil {
				t.Fatal(err)
			}
			if got := analyzer.changedItems(items); len(got) != 0 {
				t.Fatalf("successful batch still pending: %#v", got)
			}
			if _, count, _ := analyzer.Status(); count != len(items) {
				t.Fatalf("analyzed count = %d", count)
			}
		})
	}
}

func TestBatchPreservesOwnerEditsMadeDuringExtraction(t *testing.T) {
	items := movieItems(2)
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 200} })
	analyzer.extractCredits = func(_ context.Context, item library.Item, _, _ float64) ([]byte, error) {
		if err := analyzer.SetManual(item, "intro", 4, 12, 200); err != nil {
			t.Fatal(err)
		}
		if err := analyzer.Suppress(item, "credits"); err != nil {
			t.Fatal(err)
		}
		frames := make([]byte, 0, 40*creditFrameWidth*creditFrameHeight)
		for range 40 {
			frames = append(frames, creditFixture(12, 245)...)
		}
		return frames, nil
	}
	if err := analyzer.analyzeBatch(t.Context(), items); err != nil {
		t.Fatal(err)
	}
	reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)})
	for _, item := range items {
		got := reloaded.Markers(item, nil)
		if len(got) != 1 || got[0].Type != "intro" || got[0].Source != "manual" || got[0].Start != 4 || got[0].End != 12 {
			t.Fatalf("Owner edits overwritten: %#v", got)
		}
		if !reflect.DeepEqual(reloaded.records[item.ID].Suppressed, []string{"credits"}) {
			t.Fatal("suppression was lost")
		}
	}
}

func TestWorkerReportsExtractionFailureAndRecovery(t *testing.T) {
	analyzer := NewAnalyzer(Config{})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 200} })
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) {
		return nil, errors.New("unavailable")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go analyzer.work(ctx)
	analyzer.Enqueue(movieItems(1))
	waitForMarkerState(t, analyzer, "failed")
	_, count, message := analyzer.Status()
	if count != 0 || message == "" {
		t.Fatalf("failed status: %d %q", count, message)
	}
	analyzer.enqueueChanged(nil)
	if state, _, message := analyzer.Status(); state != "failed" || message == "" {
		t.Fatal("unchanged scan hid failure")
	}
	// The next job needs no extraction, allowing recovery without changing worker dependencies.
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 50} })
	analyzer.Enqueue(movieItems(1))
	waitForMarkerState(t, analyzer, "complete")
	if _, count, message := analyzer.Status(); count != 1 || message != "" {
		t.Fatalf("recovery: %d %q", count, message)
	}
}

func TestUnchangedScanDoesNotCompleteRunningAnalysis(t *testing.T) {
	analyzer := NewAnalyzer(Config{})
	items := movieItems(1)
	analyzer.records[items[0].ID] = Record{Revision: mediaRevision(items[0]), DetectorVersion: DetectorVersion}
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 200} })
	entered, release := make(chan struct{}), make(chan struct{})
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) {
		close(entered)
		<-release
		return nil, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	defer close(release)
	go analyzer.work(ctx)
	analyzer.Enqueue(items)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("analysis did not start")
	}
	analyzer.enqueueChanged(items)
	if state, _, _ := analyzer.Status(); state != "running" {
		t.Fatalf("state = %q", state)
	}
}

func TestStaleGeneratedMarkersWaitForReanalysisAndOwnerEditsDoNotMarkComplete(t *testing.T) {
	item := movieItems(1)[0]
	analyzer := NewAnalyzer(Config{})
	analyzer.records[item.ID] = Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion - 1,
		Markers: []Marker{{Type: "intro", Label: "Intro", Start: 1, End: 30, Source: "fingerprint"}}}
	if got := analyzer.Markers(item, nil); len(got) != 0 {
		t.Fatalf("stale auto-skip exposed: %#v", got)
	}
	if err := analyzer.SetManual(item, "recap", 40, 50, 200); err != nil {
		t.Fatal(err)
	}
	if err := analyzer.Suppress(item, "credits"); err != nil {
		t.Fatal(err)
	}
	if len(analyzer.changedItems([]library.Item{item})) != 1 {
		t.Fatal("Owner edit marked analysis complete")
	}
	if got := analyzer.Markers(item, nil); len(got) != 1 || got[0].Source != "manual" {
		t.Fatalf("manual marker = %#v", got)
	}
}

func TestGroupingDoesNotCountDuplicateItemsAsConsensus(t *testing.T) {
	item := movieItems(1)[0]
	groups := groupMarkerItems([]library.Item{item, item, item})
	for _, titles := range groups.movies {
		if len(titles) != 1 {
			t.Fatalf("duplicate consensus peers = %d", len(titles))
		}
	}
}

func TestFailedGroupDoesNotBlockOtherLibraries(t *testing.T) {
	items := movieItems(2)
	items[1].Library = "Other"
	analyzer := NewAnalyzer(Config{})
	analyzer.SetProbe(func(_ context.Context, item library.Item) Media {
		if item.ID == items[0].ID {
			return Media{}
		}
		return Media{Duration: 50}
	})
	if err := analyzer.analyzeBatch(t.Context(), items); err == nil {
		t.Fatal("failed group was not reported")
	}
	if _, found := analyzer.records[items[0].ID]; found {
		t.Fatal("failed group published results")
	}
	if record := analyzer.records[items[1].ID]; record.DetectorVersion != DetectorVersion {
		t.Fatal("unrelated library did not finish")
	}
}

func TestFingerprintExtractionOwnsDeadlineAndRespectsCancellation(t *testing.T) {
	calls := 0
	check := func(ctx context.Context) {
		t.Helper()
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 10*time.Minute {
			t.Fatal("unbounded extraction")
		}
		calls++
	}
	analyzer := NewAnalyzer(Config{})
	analyzer.extract = func(ctx context.Context, _ library.Item, _ string, _, _ float64) ([]uint32, error) {
		check(ctx)
		return nil, nil
	}
	analyzer.extractVisual = func(ctx context.Context, _ library.Item, _ string, _, _ float64) ([]uint64, error) {
		check(ctx)
		return nil, nil
	}
	if _, err := analyzer.fingerprint(t.Context(), library.Item{}, "opening", 0, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := analyzer.visualFingerprint(t.Context(), library.Item{}, "opening", 0, 20); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := analyzer.fingerprint(ctx, library.Item{}, "opening", 0, 20); !errors.Is(err, context.Canceled) {
		t.Fatalf("audio cancellation = %v", err)
	}
	if _, err := analyzer.visualFingerprint(ctx, library.Item{}, "opening", 0, 20); !errors.Is(err, context.Canceled) {
		t.Fatalf("visual cancellation = %v", err)
	}
	if calls != 2 {
		t.Fatalf("canceled extraction made side effects: %d calls", calls)
	}
}
