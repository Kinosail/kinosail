package watchrooms

import (
	"testing"
	"time"
)

func TestRoomsRemainingLifecycleEdges(t *testing.T) { //nolint:cyclop // One white-box test covers independent bounded lifecycle edges.
	t.Parallel()
	rooms := New(0)
	if rooms.ttl != defaultTTL {
		t.Fatalf("default TTL = %v", rooms.ttl)
	}
	identifiers := []string{"aaaaaaaaaaaa1", "aaaaaaaaaaaa2", "bbbbbbbbbbbb3"}
	rooms.newID = func() string {
		id := identifiers[0]
		identifiers = identifiers[1:]
		return id
	}
	first, ok := rooms.Create("leader", "movie", 0)
	if !ok || first != "aaaaaaaaaaaa" {
		t.Fatalf("first room = %q, %v", first, ok)
	}
	second, ok := rooms.Create("leader", "movie", 0)
	if !ok || second != "bbbbbbbbbbbb" {
		t.Fatalf("collision retry = %q, %v", second, ok)
	}

	missing := &Subscription{rooms: rooms, roomID: "missing", id: 99}
	if rooms.ObserveDrift(missing, 1) || rooms.Update(missing, Event{Action: "pause", Media: "movie"}, true) {
		t.Fatal("missing subscription was accepted")
	}
	if rooms.Update(nil, Event{}, true) {
		t.Fatal("nil subscription was accepted")
	}
	(&Subscription{}).Close()
	if current, found := rooms.active("missing", rooms.now()); found || current != nil {
		t.Fatalf("missing room = %#v, %v", current, found)
	}
	if rooms.offer(rooms.rooms[first], 99, Event{}) {
		t.Fatal("event offered to missing subscriber")
	}
	if member, found := currentSubscriber(nil, 99); found || member != (subscriber{}) {
		t.Fatalf("nil subscriber = %#v, %v", member, found)
	}

	now := time.Now()
	rooms.now = func() time.Time { return now }
	rooms.rooms[first].updated = now.Add(-defaultTTL - time.Second)
	rooms.Metrics()
	if _, found := rooms.rooms[first]; found {
		t.Fatal("metrics retained expired room")
	}
}
