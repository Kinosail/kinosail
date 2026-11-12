package operations

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

func auditStoreEvent(id, category string, created time.Time) auditjournal.Event {
	return auditjournal.Event{ID: id, Time: created.Format(time.RFC3339Nano), Category: category, Action: "activity.recorded", Result: "success"}
}

func auditStoreHTTP() auditjournal.HTTPConfig {
	return auditjournal.HTTPConfig{
		Action:        func(*http.Request) (string, string) { return "administration", "settings.changed" },
		Details:       func(*http.Request) map[string]string { return nil },
		Actor:         func(*http.Request) auditjournal.Actor { return auditjournal.Actor{Name: "Owner", ID: "owner"} },
		FallbackActor: func(*http.Request) auditjournal.Actor { return auditjournal.Actor{} },
		Target:        func(*http.Request) string { return "" },
		MarkSecurity:  func(*http.Request) {},
		RequestID:     func(*http.Request) string { return "request-1" },
		Remote:        func(*http.Request) string { return "192.0.2.1" },
		NewID:         func() string { return "generated" },
	}
}

func TestAuditStoreKeepsAuditAndPlaybackRetentionsApart(t *testing.T) {
	store := NewAuditStore(t.Context(), t.TempDir(), nil, auditStoreHTTP(), 3*time.Hour, 30*time.Minute)
	now := time.Now().UTC()
	store.Record(auditStoreEvent("audit-two-hours", "administration", now.Add(-2*time.Hour)))
	store.Record(auditStoreEvent("playback-one-hour", "playback", now.Add(-time.Hour)))
	store.Record(auditStoreEvent("audit-four-hours", "administration", now.Add(-4*time.Hour)))
	store.Record(auditStoreEvent("current", "security", now))
	kept := map[string]bool{}
	for _, event := range store.Query("", 100) {
		kept[event.ID] = true
	}
	if len(kept) != 2 || !kept["audit-two-hours"] || !kept["current"] {
		t.Fatalf("retained events = %v", kept)
	}
}

func TestAuditStoreUsesDefaultRetentionsWhenNoneAreGiven(t *testing.T) {
	store := NewAuditStore(t.Context(), t.TempDir(), nil, auditStoreHTTP())
	now := time.Now().UTC()
	store.Record(auditStoreEvent("yesterday-audit", "administration", now.Add(-24*time.Hour)))
	store.Record(auditStoreEvent("yesterday-playback", "playback", now.Add(-24*time.Hour)))
	if events := store.Query("", 100); len(events) != 2 {
		t.Fatalf("default retention dropped day-old events: %#v", events)
	}
}

func TestAuditStoreDeliversRecordedEventsAndCountsRequests(t *testing.T) {
	delivered := make(chan auditjournal.Event, 1)
	store := NewAuditStore(t.Context(), t.TempDir(), func(event auditjournal.Event) { delivered <- event }, auditStoreHTTP())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", nil)
	event := store.Event(request, "security", "session.revoked", "Owner", "owner", "success", nil)
	if event.ID != "generated" || event.RequestID != "request-1" || event.Remote != "192.0.2.1" {
		t.Fatalf("event did not use the app callbacks: %#v", event)
	}
	store.Record(event)
	select {
	case got := <-delivered:
		if got.ID != "generated" {
			t.Fatalf("delivered event = %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("recorded event was not delivered to the notifier")
	}
	store.Observe("request-1", http.MethodPost, "/settings", http.StatusInternalServerError, time.Millisecond, false)
	if requests, failed, panics := store.Counts(); requests != 1 || failed != 1 || panics != 0 {
		t.Fatalf("counts = %d, %d, %d", requests, failed, panics)
	}
}
