package auditjournal

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestJournalAtEventCapacityKeepsNewestSignedEvents(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	seedSignedJournal(t, directory, maxActivityEvents)
	journal := New(t.Context(), Config{DataDir: directory})
	now := time.Now().UTC()
	for index := range 3 {
		journal.Record(benchmarkEvent("newest-"+strconv.Itoa(index), now))
	}
	if status := journal.Status(); !status.Healthy {
		t.Fatalf("capacity journal = %#v", status)
	}
	assertNewestRetained(t, journal, "newest-2", "seed-0")
	reloaded := New(t.Context(), Config{DataDir: directory})
	if status := reloaded.Status(); !status.Healthy {
		t.Fatalf("reloaded capacity journal = %#v", status)
	}
	assertNewestRetained(t, reloaded, "newest-2", "seed-0")
}

func TestLargeEventsCompactJournalInsteadOfFailing(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	seedSignedEvents(t, directory, 1_100, 15)
	journal := New(t.Context(), Config{DataDir: directory})
	now := time.Now().UTC()
	for index := range 12 {
		event := benchmarkEvent("large-"+strconv.Itoa(index), now)
		for field := range 15 {
			event.Details["padding-"+strconv.Itoa(field)] = strings.Repeat("y", 4000)
		}
		journal.Record(event)
	}
	if status := journal.Status(); !status.Healthy {
		t.Fatalf("large-event journal = %#v", status)
	}
	info, err := os.Stat(filepath.Join(directory, "audit.jsonl"))
	if err != nil || info.Size() > maxAuditJournalBytes {
		t.Fatalf("journal size = %v, %v", info, err)
	}
	reloaded := New(t.Context(), Config{DataDir: directory})
	if status := reloaded.Status(); !status.Healthy {
		t.Fatalf("reloaded large-event journal = %#v", status)
	}
	assertNewestRetained(t, reloaded, "large-11", "seed-0")
}

func TestConcurrentRecordsKeepOneVerifiableChain(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	journal := New(t.Context(), Config{DataDir: directory})
	now := time.Now().UTC()
	var writers, readers sync.WaitGroup
	stop := make(chan struct{})
	readers.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = journal.WriteJSONL(io.Discard)
				_ = journal.Query("", 1000)
			}
		}
	})
	for writer := range 4 {
		writers.Go(func() {
			for index := range 50 {
				journal.Record(benchmarkEvent("writer-"+strconv.Itoa(writer)+"-"+strconv.Itoa(index), now))
			}
		})
	}
	writers.Wait()
	close(stop)
	readers.Wait()
	reloaded := New(t.Context(), Config{DataDir: directory})
	if status := reloaded.Status(); !status.Healthy || !journal.Status().Healthy || len(reloaded.Query("", 1000)) != 200 {
		t.Fatalf("concurrent journal = %#v, %#v, %d events", journal.Status(), status, len(reloaded.Query("", 1000)))
	}
}

func assertNewestRetained(t *testing.T, journal *Journal, newest, dropped string) {
	t.Helper()
	if events := journal.Query("", 1); len(events) != 1 || events[0].ID != newest {
		t.Fatalf("newest event = %#v", events)
	}
	var exported strings.Builder
	if err := journal.WriteJSONL(&exported); err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(exported.String(), "\n")
	if lines > maxActivityEvents || strings.Contains(exported.String(), `"id":"`+dropped+`"`) {
		t.Fatalf("retained %d events, oldest still present=%v", lines, strings.Contains(exported.String(), `"id":"`+dropped+`"`))
	}
}
