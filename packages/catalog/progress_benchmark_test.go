package catalog

import (
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

const benchmarkProfile = "0123456789abcdef0123456789abcdef"

func benchmarkProgressValues(items, watched int) map[string]PlaybackState {
	values := make(map[string]PlaybackState, watched*2)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := range watched {
		id := benchmarkItemID(index * (items / watched))
		values[benchmarkProfile+":"+id] = PlaybackState{Seconds: float64(index + 1), Updated: base.Add(time.Duration(index) * time.Minute)}
		values["fedcba9876543210fedcba9876543210:"+id] = PlaybackState{Seconds: 1, Updated: base}
	}
	return values
}

func benchmarkItemID(index int) string { return "item-" + strconv.Itoa(index) + "-0123456789abcdef" }

func benchmarkLibrary(count int) []library.Item {
	items := make([]library.Item, count)
	for index := range items {
		items[index] = library.Item{ID: benchmarkItemID(index), Title: "Title " + strconv.Itoa(index), Kind: "video"}
	}
	return items
}

func BenchmarkProgressHistory50K(b *testing.B) {
	items := benchmarkLibrary(50_000)
	values := benchmarkProgressValues(len(items), 1_000)
	b.ReportAllocs()
	for b.Loop() {
		if history := ProgressHistory(values, benchmarkProfile, false, items); len(history) != 1_000 {
			b.Fatalf("history = %d", len(history))
		}
	}
}

func BenchmarkProgressHeartbeatWithReaders20K(b *testing.B) {
	benchmarkProgressHeartbeat(b, false)
}

func BenchmarkProgressHeartbeatCoalescedWithReaders20K(b *testing.B) {
	benchmarkProgressHeartbeat(b, true)
}

func benchmarkProgressHeartbeat(b *testing.B, coalesce bool) {
	directory := b.TempDir()
	database, err := documentdb.Open(directory, true, documentdb.Config{Documents: []string{"progress.json"}})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	if err := database.SaveJSON("progress.json", benchmarkProgressValues(40_000, 10_000)); err != nil {
		b.Fatal(err)
	}
	viewer := func(*http.Request) ProgressViewer { return ProgressViewer{ID: benchmarkProfile} }
	store := NewRequestProgressStore(directory, database, progressDatabaseLoader(database), nil, viewer, nil)
	if store.Err() != nil {
		b.Fatal(store.Err())
	}
	if coalesce {
		b.Cleanup(store.StartWriteCoalescing(5 * time.Second))
	}
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for reader := range 4 {
		readers.Go(func() {
			for index := reader; ; index++ {
				select {
				case <-stop:
					return
				default:
				}
				store.GetFor(benchmarkProfile, false, benchmarkItemID(index%40_000))
			}
		})
	}
	key := benchmarkProfile + ":" + benchmarkItemID(0)
	b.ReportAllocs()
	seconds := 1.0
	for b.Loop() {
		seconds++
		if _, _, _, err := store.Update(key, ProgressRevision(seconds, nil, "session", uint64(seconds), time.Now())); err != nil {
			b.Fatal(err)
		}
	}
	close(stop)
	readers.Wait()
}
