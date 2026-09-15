// Package watchrooms coordinates synchronized playback rooms.
package watchrooms

import (
	"crypto/rand"
	"math"
	"sync"
	"time"
	"unicode"
)

const (
	defaultTTL     = 4 * time.Hour
	maxRooms       = 1024
	maxSubscribers = 64
	eventQueue     = 8
)

// MaximumSeconds bounds every shared playback position.
const MaximumSeconds = 366 * 24 * 60 * 60

// MaximumDriftSeconds bounds one client synchronization observation.
const MaximumDriftSeconds = 60 * 60

// Event is one authoritative playback state.
type Event struct {
	Action      string    `json:"action"`
	Media       string    `json:"media"`
	Seconds     float64   `json:"seconds"`
	Leader      bool      `json:"leader"`
	Revision    uint64    `json:"revision"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Drift       float64   `json:"drift,omitempty"`
}

// Metrics reports aggregate room coordination state without Viewer or media identifiers.
type Metrics struct {
	Connections       int
	Joins             uint64
	Reconnects        uint64
	SlowDrops         uint64
	DriftObservations uint64
	DriftSeconds      float64
}

// Rooms owns active room state and bounded subscriber queues.
type Rooms struct {
	mu                                              sync.Mutex
	rooms                                           map[string]*room
	ttl                                             time.Duration
	nextID                                          uint64
	now                                             func() time.Time
	newID                                           func() string
	joins, reconnects, slowDrops, driftObservations uint64
	driftSeconds                                    float64
}

type room struct {
	leader      string
	updated     time.Time
	event       Event
	subscribers map[uint64]subscriber
	seen        map[string]struct{}
}

type subscriber struct {
	viewer string
	events chan Event
}

// Subscription receives ordered room events.
type Subscription struct {
	rooms  *Rooms
	roomID string
	id     uint64
	events <-chan Event
	once   sync.Once
}

// New returns an empty room collection.
func New(ttl time.Duration) *Rooms {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Rooms{rooms: make(map[string]*room), ttl: ttl, now: time.Now, newID: rand.Text}
}

// Create starts one bounded room and returns its identifier.
func (rooms *Rooms) Create(leader, media string, seconds float64) (string, bool) {
	if !validIdentity(leader) || !validState("pause", media, seconds) {
		return "", false
	}
	now := rooms.now()
	rooms.mu.Lock()
	rooms.prune(now)
	if len(rooms.rooms) >= maxRooms {
		rooms.mu.Unlock()
		return "", false
	}
	id := rooms.newID()[:12]
	for rooms.rooms[id] != nil {
		id = rooms.newID()[:12]
	}
	rooms.rooms[id] = &room{
		leader: leader, updated: now, event: Event{Action: "pause", Media: media, Seconds: seconds, Revision: 1, EffectiveAt: now.UTC()},
		subscribers: make(map[uint64]subscriber),
		seen:        make(map[string]struct{}),
	}
	rooms.mu.Unlock()
	return id, true
}

// Snapshot returns the current state for one Viewer.
func (rooms *Rooms) Snapshot(id, viewer string) (Event, bool) {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	current, found := rooms.active(id, rooms.now())
	if !found {
		return Event{}, false
	}
	event := current.event
	event.Leader = viewer == current.leader
	return event, true
}

// Join adds one Viewer and returns the current state plus its event stream.
func (rooms *Rooms) Join(id, viewer string) (Event, *Subscription, bool) {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	current, found := rooms.active(id, rooms.now())
	if !found || !validIdentity(viewer) || len(current.subscribers) >= maxSubscribers {
		return Event{}, nil, false
	}
	rooms.nextID++
	rooms.joins++
	if _, seen := current.seen[viewer]; seen {
		rooms.reconnects++
	}
	current.seen[viewer] = struct{}{}
	events := make(chan Event, eventQueue)
	current.subscribers[rooms.nextID] = subscriber{viewer: viewer, events: events}
	current.updated = rooms.now()
	event := current.event
	event.Leader = viewer == current.leader
	return event, &Subscription{rooms: rooms, roomID: id, id: rooms.nextID, events: events}, true
}

// ObserveDrift records one bounded client synchronization difference.
func (rooms *Rooms) ObserveDrift(subscription *Subscription, seconds float64) bool {
	if subscription == nil || subscription.rooms != rooms || seconds < 0 || seconds > MaximumDriftSeconds || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return false
	}
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	current, found := rooms.active(subscription.roomID, rooms.now())
	if _, joined := currentSubscriber(current, subscription.id); !found || !joined {
		return false
	}
	rooms.driftObservations++
	rooms.driftSeconds = seconds
	return true
}

// Metrics returns bounded aggregate room metrics.
func (rooms *Rooms) Metrics() Metrics {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	rooms.prune(rooms.now())
	connections := 0
	for _, current := range rooms.rooms {
		connections += len(current.subscribers)
	}
	return Metrics{connections, rooms.joins, rooms.reconnects, rooms.slowDrops, rooms.driftObservations, rooms.driftSeconds}
}

// Update accepts one authorized event and queues the resulting state.
func (rooms *Rooms) Update(subscription *Subscription, event Event, authorized bool) bool { //nolint:cyclop // One locked transition preserves ordering, authorization, revision, and queue invariants.
	if subscription == nil || subscription.rooms != rooms {
		return false
	}
	now := rooms.now()
	rooms.mu.Lock()
	current, found := rooms.active(subscription.roomID, now)
	member, joined := currentSubscriber(current, subscription.id)
	if !found || !joined {
		rooms.mu.Unlock()
		return false
	}
	if member.viewer != current.leader {
		denied := Event{Action: "denied", Revision: current.event.Revision, EffectiveAt: now.UTC()}
		keep := rooms.offer(current, subscription.id, denied)
		rooms.mu.Unlock()
		return keep
	}
	if !authorized || !validState(event.Action, event.Media, event.Seconds) || event.Revision != 0 && event.Revision <= current.event.Revision {
		rooms.mu.Unlock()
		return true
	}
	event.Revision, event.EffectiveAt = current.event.Revision+1, now.UTC()
	current.event, current.updated = event, now
	for id, target := range current.subscribers {
		event.Leader = target.viewer == current.leader
		rooms.offer(current, id, event)
	}
	_, keep := current.subscribers[subscription.id]
	rooms.mu.Unlock()
	return keep
}

// Events returns the ordered event stream.
func (subscription *Subscription) Events() <-chan Event { return subscription.events }

// Close leaves the room and closes the event stream.
func (subscription *Subscription) Close() {
	if subscription == nil || subscription.rooms == nil {
		return
	}
	subscription.once.Do(func() { subscription.rooms.leave(subscription.roomID, subscription.id) })
}

func (rooms *Rooms) active(id string, now time.Time) (*room, bool) {
	current, found := rooms.rooms[id]
	if !found {
		return nil, false
	}
	if now.Sub(current.updated) <= rooms.ttl {
		return current, true
	}
	rooms.close(current)
	delete(rooms.rooms, id)
	return nil, false
}

func (rooms *Rooms) prune(now time.Time) {
	for id, current := range rooms.rooms {
		if now.Sub(current.updated) > rooms.ttl {
			rooms.close(current)
			delete(rooms.rooms, id)
		}
	}
}

func (rooms *Rooms) offer(current *room, id uint64, event Event) bool {
	target, found := current.subscribers[id]
	if !found {
		return false
	}
	select {
	case target.events <- event:
		return true
	default:
		close(target.events)
		delete(current.subscribers, id)
		rooms.slowDrops++
		return false
	}
}

func (rooms *Rooms) leave(roomID string, id uint64) {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	if current := rooms.rooms[roomID]; current != nil {
		if target, found := current.subscribers[id]; found {
			close(target.events)
			delete(current.subscribers, id)
		}
	}
}

func (rooms *Rooms) close(current *room) {
	for id, target := range current.subscribers {
		close(target.events)
		delete(current.subscribers, id)
	}
}

func currentSubscriber(current *room, id uint64) (subscriber, bool) {
	if current == nil {
		return subscriber{}, false
	}
	target, found := current.subscribers[id]
	return target, found
}

func validState(action, media string, seconds float64) bool {
	return (action == "play" || action == "pause" || action == "seek" || action == "media") && validIdentity(media) && seconds >= 0 && seconds <= MaximumSeconds && !math.IsNaN(seconds) && !math.IsInf(seconds, 0)
}

func validIdentity(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
