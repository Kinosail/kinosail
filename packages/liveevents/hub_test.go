package liveevents

import (
	"testing"
	"time"
)

func TestHubScopesAndReplaysEvents(t *testing.T) {
	hub := New()
	hub.now = func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.FixedZone("test", 60)) }
	hub.Publish("viewer-a", "download.updated", "/api/v1/downloads/a")
	hub.Publish("viewer-b", "download.updated", "/api/v1/downloads/b")
	hub.Publish("", "library.updated", "/api/v1/library")

	backlog, initialSubscription, ok := hub.subscribe("viewer-a", 0)
	if !ok || len(backlog) != 2 || backlog[0].Resource != "/api/v1/downloads/a" || backlog[1].Type != "library.updated" || backlog[0].At.Location() != time.UTC {
		t.Fatalf("scoped backlog = %+v, accepted=%t", backlog, ok)
	}
	resumed, resumedSubscription, ok := hub.subscribe("viewer-a", backlog[0].ID)
	if !ok || len(resumed) != 1 || resumed[0].Type != "library.updated" {
		t.Fatalf("resumed backlog = %+v, accepted=%t", resumed, ok)
	}
	resumedSubscription.close()
	initialSubscription.close()
	initialSubscription.close()
	var missing *subscription
	missing.close()
	(&subscription{}).close()
}

func TestHubBoundsHistory(t *testing.T) {
	hub := New()
	for id := 0; id <= historyLimit; id++ {
		hub.Publish("", "library.updated", "/api/v1/library")
	}
	if len(hub.history) != historyLimit || hub.history[0].ID != 2 {
		t.Fatalf("bounded history = %d items starting at %d", len(hub.history), hub.history[0].ID)
	}
}

func TestHubTracksReconnectsAndSlowStreams(t *testing.T) {
	hub := New()
	_, current, ok := hub.subscribe("viewer", 0)
	if !ok {
		t.Fatal("initial subscription was rejected")
	}
	_, resumed, ok := hub.subscribe("viewer", 7)
	if !ok {
		t.Fatal("resumed subscription was rejected")
	}
	for event := 0; event <= subscriberQueue; event++ {
		hub.Publish("other", "download.updated", "/api/v1/downloads")
	}
	for event := 0; event <= subscriberQueue; event++ {
		hub.Publish("viewer", "download.updated", "/api/v1/downloads")
	}
	metrics := hub.Metrics()
	if metrics.Published != 34 || metrics.Reconnects != 1 || metrics.SlowDrops != 2 || metrics.Subscribers != 0 {
		t.Fatalf("live event metrics = %+v", metrics)
	}
	current.close()
	resumed.close()
}

func TestHubRejectsExcessStreams(t *testing.T) {
	full := New()
	for id := 0; id < subscriberLimit; id++ {
		if _, _, accepted := full.subscribe("viewer", 0); !accepted {
			t.Fatalf("subscription %d was rejected", id)
		}
	}
	if backlog, subscription, accepted := full.subscribe("viewer", 0); accepted || backlog != nil || subscription != nil {
		t.Fatalf("excess subscription = %+v, %+v, %t", backlog, subscription, accepted)
	}
	if metrics := full.Metrics(); metrics.Rejected != 1 || metrics.Subscribers != subscriberLimit {
		t.Fatalf("capacity metrics = %+v", metrics)
	}
}
