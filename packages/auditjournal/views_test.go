package auditjournal

import (
	"bytes"
	"testing"
	"time"
)

func TestQueriesAndNotificationsCannotMutateJournal(t *testing.T) {
	notified := make(chan Event, 1)
	journal := New(t.Context(), Config{Notify: func(event Event) {
		event.Details["safe"] = "changed-by-notifier"
		notified <- event
	}})
	event := testEvent("event", "security", time.Now().UTC())
	event.Details = map[string]string{"safe": "yes"}
	journal.Record(event)
	waitForEvent(t, notified)
	event.Details["safe"] = "changed"
	queried := journal.Query("", 10)
	queried[0].Details["safe"] = "changed-again"
	var output bytes.Buffer
	if err := journal.WriteJSONL(&output); err != nil || !bytes.Contains(output.Bytes(), []byte(`"safe":"yes"`)) {
		t.Fatalf("journal was mutated: %s, %v", output.String(), err)
	}
}

func TestPlayerViewsPreserveOrderingFiltersAndTripwireWarnings(t *testing.T) {
	journal := New(t.Context(), Config{})
	now := time.Now().UTC()
	journal.Record(testEvent("playback", "playback", now))
	probe := testEvent("probe", "security", now)
	probe.Action = "security.tripwire"
	probe.Details = map[string]string{"quarantined": "false"}
	journal.Record(probe)
	if warning := journal.TripwireWarning(); warning != "Public scanner probe detected; local and WireGuard access remain available." {
		t.Fatalf("probe warning = %q", warning)
	}
	quarantined := testEvent("quarantined", "security", now)
	quarantined.Action = "security.tripwire"
	quarantined.Details = map[string]string{"quarantined": "true"}
	journal.Record(quarantined)
	if warning := journal.TripwireWarning(); warning != "Public scanner quarantined; local and WireGuard access remain available." {
		t.Fatalf("quarantine warning = %q", warning)
	}
	logs := journal.RecentLogs()
	security := journal.Query("security", 10)
	if len(logs) != 2 || logs[0].ID != "quarantined" || len(security) != 2 || security[0].ID != "quarantined" {
		t.Fatalf("logs = %#v, security = %#v", logs, security)
	}
}

func waitForEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("notification was not delivered")
	}
}
