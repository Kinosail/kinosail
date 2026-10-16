package documentdb

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

type benchmarkProgress struct {
	Seconds float64   `json:"seconds"`
	Updated time.Time `json:"updated"`
	Session string    `json:"session"`
}

func benchmarkEntryStore(b *testing.B, entries int) *Store {
	b.Helper()
	store, err := Open(b.TempDir(), true, Config{Documents: []string{"progress.json"}})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = store.Close() })
	values := make(map[string]benchmarkProgress, entries)
	for index := range entries {
		values["viewer:"+strconv.Itoa(index)] = benchmarkProgress{Seconds: float64(index), Updated: time.Unix(int64(index), 0).UTC(), Session: "session"}
	}
	if err := store.SaveJSON("progress.json", values); err != nil {
		b.Fatal(err)
	}
	if err := store.EnableEntries("progress.json"); err != nil {
		b.Fatal(err)
	}
	return store
}

func BenchmarkSaveEntry20K(b *testing.B) {
	store := benchmarkEntryStore(b, 20_000)
	b.ReportAllocs()
	seconds := 0.0
	for b.Loop() {
		seconds++
		if err := store.SaveEntry("progress.json", "viewer:1", benchmarkProgress{Seconds: seconds, Updated: time.Now().UTC(), Session: "session"}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSaveEntryDuringExport20K(b *testing.B) {
	store := benchmarkEntryStore(b, 20_000)
	stop := make(chan struct{})
	var exporters sync.WaitGroup
	exporters.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := store.Export(); err != nil {
				b.Error(err)
				return
			}
		}
	})
	b.ReportAllocs()
	seconds := 0.0
	for b.Loop() {
		seconds++
		if err := store.SaveEntry("progress.json", "viewer:1", benchmarkProgress{Seconds: seconds, Updated: time.Now().UTC(), Session: "session"}); err != nil {
			b.Fatal(err)
		}
	}
	close(stop)
	exporters.Wait()
}

func BenchmarkExport20K(b *testing.B) {
	store := benchmarkEntryStore(b, 20_000)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := store.Export(); err != nil {
			b.Fatal(err)
		}
	}
}
