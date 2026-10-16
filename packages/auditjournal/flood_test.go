package auditjournal

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRepeatedSecurityEventsFromOneSourceAreAggregated(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	clock := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	remote := "203.0.113.9"
	sequence := 0
	tracker := NewHTTPTracker(New(t.Context(), Config{DataDir: directory}), HTTPConfig{
		Action:        func(*http.Request) (string, string) { return "", "" },
		Details:       func(*http.Request) map[string]string { return nil },
		Actor:         func(*http.Request) Actor { return Actor{} },
		FallbackActor: func(*http.Request) Actor { return Actor{} },
		Target:        func(request *http.Request) string { return request.URL.Path },
		MarkSecurity:  func(*http.Request) {},
		RequestID:     func(*http.Request) string { return "request" },
		Remote:        func(*http.Request) string { return remote },
		NewID:         func() string { sequence++; return "event-" + strconv.Itoa(sequence) },
		Now:           func() time.Time { return clock },
	})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	for range 200 {
		tracker.Denied(request, "authentication required")
	}
	for index := range 50 {
		remote = "2001:db8:1:2::" + strconv.FormatInt(int64(index+1), 16)
		tracker.Denied(request, "authentication required")
	}
	remote = "203.0.113.9"
	tracker.Tripwire(request, "scanner probe", true)
	remote = "198.51.100.7"
	tracker.Denied(request, "authentication required")
	events := tracker.Query("security", 1000)
	counts := map[string]int{}
	for _, event := range events {
		counts[event.Action+" "+event.Remote]++
	}
	if counts["access.denied 203.0.113.9"] != 20 || len(events) != 20+20+1+1 || counts["security.tripwire 203.0.113.9"] != 1 || counts["access.denied 198.51.100.7"] != 1 {
		t.Fatalf("aggregated counts = %v (%d events)", counts, len(events))
	}
	if tracker.TripwireWarning() == "" {
		t.Fatal("aggregation hid the scanner quarantine")
	}
	data, err := os.ReadFile(filepath.Join(directory, "audit.jsonl"))
	if err != nil || bytes.Count(data, []byte("\n")) != len(events) {
		t.Fatalf("journal lines = %d, events = %d, %v", bytes.Count(data, []byte("\n")), len(events), err)
	}
	clock = clock.Add(61 * time.Second)
	remote = "203.0.113.9"
	tracker.Denied(request, "authentication required")
	latest := tracker.Query("security", 3)
	summaries := map[string]string{}
	for _, event := range latest[1:] {
		summaries[event.Remote] = event.Details["suppressed"]
	}
	if latest[0].Remote != remote || latest[0].Details["suppressed"] != "" || summaries["203.0.113.9"] != "180" || summaries["2001:db8:1:2::32"] != "30" {
		t.Fatalf("summary events = %#v", latest)
	}
	if reloaded := New(t.Context(), Config{DataDir: directory}); !reloaded.Status().Healthy || len(reloaded.Query("security", 1000)) != len(events)+3 {
		t.Fatalf("reloaded aggregated journal = %#v", reloaded.Status())
	}
}
