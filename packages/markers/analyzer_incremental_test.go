package markers

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

type extractionCounter struct {
	mu    sync.Mutex
	calls map[string]int
	held  atomic.Int32
}

func (counter *extractionCounter) record(t testing.TB, kind string) {
	t.Helper()
	if counter.held.Load() != 1 {
		t.Errorf("%s extraction ran while %d background slots were held", kind, counter.held.Load())
	}
	counter.mu.Lock()
	counter.calls[kind]++
	counter.mu.Unlock()
}

func (counter *extractionCounter) total() int {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	total := 0
	for _, calls := range counter.calls {
		total += calls
	}
	return total
}

func countingAnalyzer(t testing.TB, failing string) (*Analyzer, *extractionCounter) {
	t.Helper()
	counter := &extractionCounter{calls: make(map[string]int)}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir(), CacheDir: t.TempDir(), Acquire: func(context.Context) (func(), error) {
		counter.held.Add(1)
		return func() { counter.held.Add(-1) }, nil
	}})
	analyzer.SetProbe(func(ctx context.Context, _ library.Item) Media {
		if counter.held.Load() != 0 {
			t.Error("media inspection ran while holding a background slot")
		}
		if !workload.BackgroundPriority(ctx) {
			t.Error("media inspection ran at playback priority")
		}
		return Media{Duration: 7200}
	})
	analyzer.extract = func(_ context.Context, item library.Item, window string, _, _ float64) ([]uint32, error) {
		counter.record(t, "audio")
		return sequence(300, uint32(item.Size)*100000), nil //nolint:gosec // Fixture sizes are bounded.
	}
	analyzer.extractVisual = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint64, error) {
		counter.record(t, "visual")
		return visualSequence(300, uint64(item.Size)*100000), nil //nolint:gosec // Fixture sizes are bounded.
	}
	analyzer.extractCredits = func(_ context.Context, item library.Item, _, _ float64) ([]byte, error) {
		counter.record(t, "credits")
		if item.ID == failing {
			return nil, errors.New("decoder failed")
		}
		return nil, nil
	}
	return analyzer, counter
}

func libraryMovies(count int) []library.Item {
	items := make([]library.Item, count)
	for index := range items {
		items[index] = library.Item{ID: fmt.Sprintf("movie-%04d", index), Kind: "video", Library: "Movies", Size: int64(index + 1), Added: time.Unix(int64(index+1), 0)}
	}
	return items
}

func runChanged(t *testing.T, analyzer *Analyzer, items []library.Item) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		analyzer.work(ctx)
		close(done)
	}()
	analyzer.enqueueChanged(items)
	deadline := time.Now().Add(10 * time.Second)
	for {
		state, _, _ := analyzer.Status()
		if state == "complete" || state == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("analysis state = %s", state)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
}

func TestAddingOneMovieAnalyzesOnlyThatMovie(t *testing.T) {
	analyzer, counter := countingAnalyzer(t, "")
	items := libraryMovies(20)
	runChanged(t, analyzer, items[:19])
	before := counter.total()
	runChanged(t, analyzer, items)
	if added := counter.total() - before; added != 3 {
		t.Fatalf("adding one movie ran %d extractions, want 3 (audio, visual, credits): %#v", added, counter.calls)
	}
	if pending := analyzer.changedItems(items); len(pending) != 0 {
		t.Fatalf("pending after analysis = %d", len(pending))
	}
}

func TestOneFailingMovieIsRecordedWithoutRepeatingTheGroup(t *testing.T) {
	analyzer, counter := countingAnalyzer(t, "movie-0001")
	items := libraryMovies(4)
	runChanged(t, analyzer, items)
	for _, item := range items {
		if record, found := analyzer.records[item.ID]; !found || record.Revision != mediaRevision(item) {
			t.Fatalf("%s record = %#v, %t", item.ID, record, found)
		}
	}
	if pending := analyzer.changedItems(items); len(pending) != 0 {
		t.Fatalf("failed item keeps the group pending: %d items", len(pending))
	}
	reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)})
	if pending := reloaded.changedItems(items); len(pending) != 0 {
		t.Fatalf("reloaded analyzer would rerun %d items", len(pending))
	}
	before := counter.total()
	runChanged(t, analyzer, items)
	if counter.total() != before {
		t.Fatalf("unchanged library reran %d extractions", counter.total()-before)
	}
}

func TestInterruptedInspectionIsRetriedWhileBrokenMediaBacksOff(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		media   Media
		pending bool
	}{
		{"interrupted", Media{Interrupted: true}, true},
		{"broken", Media{}, false},
	} {
		for _, show := range []string{"", "Show"} {
			t.Run(scenario.name+"/"+cmp.Or(show, "movies"), func(t *testing.T) {
				analyzer, _ := countingAnalyzer(t, "")
				items := libraryMovies(4)
				for index := range items {
					items[index].Show, items[index].Season, items[index].Episode = show, 1, index+1
				}
				unavailable := items[1]
				var recovered atomic.Bool
				analyzer.SetProbe(func(_ context.Context, item library.Item) Media {
					if item.ID == unavailable.ID && !recovered.Load() {
						return scenario.media
					}
					return Media{Duration: 7200}
				})
				var logs bytes.Buffer
				previous := slog.Default()
				slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
				runChanged(t, analyzer, items)
				slog.SetDefault(previous)
				deferred := map[bool]string{true: `"failed":0,"deferred":1`, false: `"failed":1,"deferred":0`}[scenario.pending]
				if !strings.Contains(logs.String(), `"level":"WARN","msg":"playback marker analysis skipped titles","operation":"markers.analyze","outcome":"partial",`+deferred) {
					t.Errorf("analysis log = %s", logs.String())
				}
				pending := analyzer.changedItems(items)
				retried := slices.ContainsFunc(pending, func(item library.Item) bool { return item.ID == unavailable.ID })
				if record := analyzer.records[unavailable.ID]; retried != scenario.pending || (record.Failed != 0) == scenario.pending {
					t.Fatalf("pending=%t record=%#v", retried, record)
				}
				if reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)}); len(reloaded.changedItems(items)) != len(pending) {
					t.Fatalf("reloaded pending = %d, want %d", len(reloaded.changedItems(items)), len(pending))
				}
				if !scenario.pending {
					return
				}
				recovered.Store(true)
				runChanged(t, analyzer, items)
				if record := analyzer.records[unavailable.ID]; record.Revision != mediaRevision(unavailable) || record.Failed != 0 {
					t.Fatalf("recovered record = %#v", record)
				}
				if pending := analyzer.changedItems(items); len(pending) != 0 {
					t.Fatalf("pending after recovery = %d", len(pending))
				}
			})
		}
	}
}
