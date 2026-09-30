package mcpgateway

import (
	"strings"
	"testing"
)

func TestEventsRejectConflictingPersistedAuthority(t *testing.T) {
	fixture := newEventFixture(t)
	fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.connections.mu.Lock()
	baseline := fixture.connections.stateLocked()
	fixture.connections.mu.Unlock()
	for name, change := range map[string]func(*eventSubscription){
		"unknown authority":       func(s *eventSubscription) { s.Scope = "unknown:viewer" },
		"grant missing":           func(s *eventSubscription) { s.Grant = "" },
		"grant mismatch":          func(s *eventSubscription) { s.Scope = "grant:other:viewer" },
		"grant external expiry":   func(s *eventSubscription) { s.AuthorityExpires = s.Expires },
		"host with grant":         func(s *eventSubscription) { s.Scope = "host:viewer" },
		"host without Owner":      func(s *eventSubscription) { s.Scope, s.Grant = "host:viewer", "" },
		"external without expiry": func(s *eventSubscription) { s.Scope, s.Grant = "external:client:viewer", "" },
		"external outlives token": func(s *eventSubscription) {
			s.Scope, s.Grant, s.AuthorityExpires = "external:client:viewer", "", s.Expires-1
		},
		"external oversized client": func(s *eventSubscription) {
			s.Scope, s.Grant, s.AuthorityExpires = "external:"+strings.Repeat("x", 2049)+":viewer", "", s.Expires
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := baseline
			state.Events = make(map[string]eventSubscription)
			for _, subscription := range baseline.Events {
				change(&subscription)
				subscription.ID = eventID(subscription)
				state.Events[subscription.ID] = subscription
			}
			store := &memoryState{}
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			loaded := testConnections("https://kino.test", fixture.principals, store)
			if loaded.err == nil || len(loaded.events) != 0 {
				t.Fatal("conflicting persisted authority created subscriptions")
			}
		})
	}
}
