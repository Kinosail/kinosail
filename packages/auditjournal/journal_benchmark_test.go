package auditjournal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func BenchmarkJournalRecord(b *testing.B) {
	for _, size := range []int{1_000, 10_000, maxActivityEvents} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			directory := b.TempDir()
			seedSignedJournal(b, directory, size)
			journal := New(b.Context(), Config{DataDir: directory})
			if !journal.Status().Healthy {
				b.Fatalf("seeded journal = %#v", journal.Status())
			}
			now := time.Now().UTC()
			index := 0
			b.ReportAllocs()
			for b.Loop() {
				journal.Record(benchmarkEvent("record-"+strconv.Itoa(index), now))
				index++
			}
			if status := journal.Status(); !status.Healthy {
				b.Fatalf("benchmark journal = %#v", status)
			}
		})
	}
}

func seedSignedJournal(tb testing.TB, directory string, count int) {
	tb.Helper()
	seedSignedEvents(tb, directory, count, 0)
}

func seedSignedEvents(tb testing.TB, directory string, count, paddingFields int) {
	tb.Helper()
	keyed := New(tb.Context(), Config{DataDir: directory})
	if !keyed.Status().Healthy {
		tb.Fatalf("journal key = %#v", keyed.Status())
	}
	now := time.Now().UTC()
	data := make([]byte, 0, count*512)
	previous := ""
	for index := range count {
		event := benchmarkEvent("seed-"+strconv.Itoa(index), now.Add(time.Duration(index-count)*time.Millisecond))
		for field := range paddingFields {
			event.Details["padding-"+strconv.Itoa(field)] = strings.Repeat("x", 4000)
		}
		event.Previous = previous
		event.Integrity = keyed.sign(event)
		previous = event.Integrity
		line, err := json.Marshal(event)
		if err != nil {
			tb.Fatal(err)
		}
		data = append(append(data, line...), '\n')
	}
	if err := os.WriteFile(filepath.Join(directory, "audit.jsonl"), data, 0o600); err != nil {
		tb.Fatal(err)
	}
}

func benchmarkEvent(id string, created time.Time) Event {
	return Event{ID: id, Time: created.Format(time.RFC3339Nano), Category: "administration", Action: "settings.server.updated", Result: "success", RequestID: "request-" + id, Remote: "203.0.113.7", Target: "/api/v1/library", Details: map[string]string{"reason": "authentication required", "method": "GET", "path": "/api/v1/library"}}
}
