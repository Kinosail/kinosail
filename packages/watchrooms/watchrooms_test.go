package watchrooms

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRoomsCoordinateLeaderAndViewerState(t *testing.T) { //nolint:cyclop,gocognit // One interface test covers the complete room coordination lifecycle.
	t.Parallel()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	rooms := New(time.Hour)
	rooms.now = func() time.Time { return now }
	id, ok := rooms.Create("leader", "movie", 12)
	if !ok || id == "" {
		t.Fatal("Create() rejected valid room")
	}
	leaderState, leader, ok := rooms.Join(id, "leader")
	if !ok || !leaderState.Leader || leaderState.Revision != 1 {
		t.Fatalf("leader Join() = %#v, %v", leaderState, ok)
	}
	viewerState, viewer, ok := rooms.Join(id, "viewer")
	if !ok || viewerState.Leader || viewerState.Media != "movie" {
		t.Fatalf("Viewer Join() = %#v, %v", viewerState, ok)
	}
	if !rooms.Update(viewer, Event{Action: "play", Media: "movie", Seconds: 20}, true) {
		t.Fatal("Viewer was disconnected after denied update")
	}
	if denied := receive(t, viewer.Events()); denied.Action != "denied" || denied.Leader {
		t.Fatalf("Viewer reply = %#v", denied)
	}
	assertNoEvent(t, leader.Events())

	now = now.Add(time.Second)
	if !rooms.Update(leader, Event{Action: "play", Media: "movie", Seconds: 20}, true) {
		t.Fatal("leader update failed")
	}
	if event := receive(t, leader.Events()); !event.Leader || event.Revision != 2 || event.EffectiveAt != now {
		t.Fatalf("leader event = %#v", event)
	}
	if event := receive(t, viewer.Events()); event.Leader || event.Revision != 2 || event.Seconds != 20 {
		t.Fatalf("Viewer event = %#v", event)
	}
	if !rooms.Update(leader, Event{Action: "pause", Media: "movie", Revision: 2}, true) {
		t.Fatal("stale update disconnected leader")
	}
	if !rooms.Update(leader, Event{Action: "media", Media: "hidden"}, false) {
		t.Fatal("unauthorized update disconnected leader")
	}
	assertNoEvent(t, viewer.Events())
	state, found := rooms.Snapshot(id, "viewer")
	if !found || state.Action != "play" || state.Revision != 2 || state.Leader {
		t.Fatalf("Snapshot() = %#v, %v", state, found)
	}
	leader.Close()
	viewer.Close()
}

func TestDeniedUpdateDoesNotEchoUntrustedPayload(t *testing.T) {
	t.Parallel()
	rooms := New(time.Hour)
	id, ok := rooms.Create("leader", "movie", 0)
	if !ok {
		t.Fatal("Create() rejected valid room")
	}
	_, viewer, ok := rooms.Join(id, "viewer")
	if !ok {
		t.Fatal("Join() rejected Viewer")
	}
	if _, _, joined := rooms.Join(id, strings.Repeat("v", 129)); joined {
		t.Fatal("Join() accepted oversized Viewer identity")
	}
	untrusted := Event{Action: strings.Repeat("a", 1000), Media: strings.Repeat("m", 1000), Seconds: math.Inf(1), Revision: ^uint64(0)}
	if !rooms.Update(viewer, untrusted, false) {
		t.Fatal("denied update disconnected Viewer")
	}
	denied := receive(t, viewer.Events())
	if denied.Action != "denied" || denied.Media != "" || denied.Seconds != 0 || denied.Revision != 1 || denied.Leader {
		t.Fatalf("denied event echoed input: %#v", denied)
	}
}

func TestRoomsBoundCapacityQueuesAndExpiry(t *testing.T) { //nolint:cyclop // One resource test covers room, subscriber, queue, and lifetime bounds.
	t.Parallel()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	rooms := New(time.Minute)
	rooms.now = func() time.Time { return now }
	id, ok := rooms.Create("leader", "movie", 0)
	if !ok {
		t.Fatal("Create() rejected valid room")
	}
	_, leader, ok := rooms.Join(id, "leader")
	if !ok {
		t.Fatal("Join() rejected leader")
	}
	for viewer := 1; viewer < maxSubscribers; viewer++ {
		if _, _, ok = rooms.Join(id, "viewer-"+strconv.Itoa(viewer)); !ok {
			t.Fatalf("Join() rejected subscriber %d", viewer)
		}
	}
	if _, _, ok = rooms.Join(id, "overflow"); ok {
		t.Fatal("Join() exceeded subscriber capacity")
	}
	for revision := range eventQueue {
		if !rooms.Update(leader, Event{Action: "seek", Media: "movie", Seconds: float64(revision)}, true) {
			t.Fatalf("Update() rejected queued event %d", revision)
		}
	}
	if rooms.Update(leader, Event{Action: "seek", Media: "movie", Seconds: eventQueue}, true) {
		t.Fatal("slow leader remained connected after queue saturation")
	}
	for range eventQueue {
		receive(t, leader.Events())
	}
	if _, open := <-leader.Events(); open {
		t.Fatal("saturated subscription remained open")
	}

	_, viewer, ok := rooms.Join(id, "viewer-after-overflow")
	if !ok {
		t.Fatal("disconnected subscriber did not release capacity")
	}
	now = now.Add(time.Minute + time.Nanosecond)
	if _, found := rooms.Snapshot(id, "viewer"); found {
		t.Fatal("expired room remained available")
	}
	if _, open := <-viewer.Events(); open {
		t.Fatal("expired room retained subscriber")
	}
}

func TestRoomsRejectInvalidStateWithoutUsingCapacity(t *testing.T) {
	t.Parallel()
	rooms := New(time.Hour)
	for _, input := range []struct {
		leader, media string
		seconds       float64
	}{{"", "movie", 0}, {strings.Repeat("l", 129), "movie", 0}, {"leader", "", 0}, {"leader", "movie\n", 0}, {"leader", "movie", -1}, {"leader", "movie", MaximumSeconds + 1}, {"leader", "movie", math.NaN()}, {"leader", "movie", math.Inf(1)}} {
		if _, ok := rooms.Create(input.leader, input.media, input.seconds); ok {
			t.Fatalf("Create(%q, %q, %v) accepted invalid state", input.leader, input.media, input.seconds)
		}
	}
	for range maxRooms {
		if _, ok := rooms.Create("leader", "movie", 0); !ok {
			t.Fatal("invalid state consumed room capacity")
		}
	}
	if _, ok := rooms.Create("leader", "movie", 0); ok {
		t.Fatal("Create() exceeded room capacity")
	}
}

func TestRoomMetricsTrackReconnectsAndBoundedDrift(t *testing.T) { //nolint:cyclop // One metrics test covers its complete connection lifecycle.
	t.Parallel()
	rooms := New(time.Hour)
	id, ok := rooms.Create("leader", "movie", 0)
	if !ok {
		t.Fatal("Create() rejected valid room")
	}
	_, first, ok := rooms.Join(id, "viewer")
	if !ok {
		t.Fatal("Join() rejected Viewer")
	}
	first.Close()
	_, resumed, ok := rooms.Join(id, "viewer")
	if !ok {
		t.Fatal("reconnect rejected Viewer")
	}
	defer resumed.Close()
	if !rooms.ObserveDrift(resumed, 1.25) {
		t.Fatal("valid drift observation was rejected")
	}
	for _, invalid := range []float64{-1, MaximumDriftSeconds + 1, math.NaN(), math.Inf(1)} {
		if rooms.ObserveDrift(resumed, invalid) {
			t.Fatalf("invalid drift %v was accepted", invalid)
		}
	}
	metrics := rooms.Metrics()
	if metrics.Connections != 1 || metrics.Joins != 2 || metrics.Reconnects != 1 || metrics.DriftObservations != 1 || metrics.DriftSeconds != 1.25 {
		t.Fatalf("room metrics = %+v", metrics)
	}
}

func receive(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event, open := <-events:
		if !open {
			t.Fatal("event stream closed early")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("event was not delivered")
		return Event{}
	}
}

func assertNoEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected event: %#v", event)
	default:
	}
}
