package auditjournal

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type floodClock struct {
	now    time.Time
	remote string
}

func floodTracker(t *testing.T, clock *floodClock) (*HTTPTracker, *http.Request) {
	t.Helper()
	sequence := 0
	tracker := NewHTTPTracker(New(t.Context(), Config{DataDir: t.TempDir()}), HTTPConfig{
		Action:        func(*http.Request) (string, string) { return "", "" },
		Details:       func(*http.Request) map[string]string { return nil },
		Actor:         func(*http.Request) Actor { return Actor{} },
		FallbackActor: func(*http.Request) Actor { return Actor{} },
		Target:        func(request *http.Request) string { return request.URL.Path },
		MarkSecurity:  func(*http.Request) {},
		RequestID:     func(*http.Request) string { return "request" },
		Remote:        func(*http.Request) string { return clock.remote },
		NewID:         func() string { sequence++; return "event-" + strconv.Itoa(sequence) },
		Now:           func() time.Time { return clock.now },
	})
	return tracker, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
}

func deniedFrom(tracker *HTTPTracker, request *http.Request, clock *floodClock, remote string, count int) {
	clock.remote = remote
	for range count {
		tracker.Denied(request, "authentication required")
	}
}

func journaledFrom(tracker *HTTPTracker, remote string) (events int, suppressed []string) {
	for _, event := range tracker.Query("security", 1000) {
		if event.Remote != remote {
			continue
		}
		events++
		if count, found := event.Details[suppressedDetail]; found {
			suppressed = append(suppressed, count)
		}
	}
	return events, suppressed
}

func TestFloodTableExhaustionStillJournalsNewSources(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	clock := &floodClock{now: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)}
	tracker, request := floodTracker(t, clock)
	for source := range trackedSecuritySources + 20 {
		deniedFrom(tracker, request, clock, "2001:db8:1:"+strconv.FormatInt(int64(source), 16)+"::1", 1)
	}
	deniedFrom(tracker, request, clock, "198.51.100.77", 50)
	deniedFrom(tracker, request, clock, "2001:db8:2::7", 50)
	for _, remote := range []string{"198.51.100.77", "2001:db8:2::7"} {
		if recorded, _ := journaledFrom(tracker, remote); recorded != securityEventBurst {
			t.Fatalf("new source %s after the source table filled: %d of 50 journaled", remote, recorded)
		}
	}
	if strings.Count(logs.String(), "activity journal is aggregating security events by network") != 1 || !strings.Contains(logs.String(), `"level":"WARN"`) || strings.Contains(logs.String(), "198.51.100.77") {
		t.Fatalf("overflow diagnostics = %q", logs.String())
	}
}

func TestSuppressedSecurityEventsAreSummarizedAfterTheSourceStops(t *testing.T) {
	t.Parallel()
	clock := &floodClock{now: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)}
	tracker, request := floodTracker(t, clock)
	deniedFrom(tracker, request, clock, "203.0.113.9", 1000)
	clock.now = clock.now.Add(securityEventWindow + time.Second)
	deniedFrom(tracker, request, clock, "198.51.100.7", 1)
	if recorded, suppressed := journaledFrom(tracker, "203.0.113.9"); recorded != securityEventBurst+1 || len(suppressed) != 1 || suppressed[0] != "980" {
		t.Fatalf("stopped source journaled %d events with suppressed counts %v", recorded, suppressed)
	}
}
