// Package liveevents provides the Player server-sent event stream.
package liveevents

import (
	"sync"
	"time"
)

const (
	historyLimit    = 128
	subscriberLimit = 256
	subscriberQueue = 16
)

// Event is one versioned server event.
type Event struct {
	ID       uint64    `json:"id"`
	Type     string    `json:"type"`
	Resource string    `json:"resource"`
	At       time.Time `json:"at"`
}

type scopedEvent struct {
	Event
	profile string
}

type subscription struct {
	hub    *Hub
	id     uint64
	events <-chan Event
}

// Metrics describes live stream use and pressure.
type Metrics struct {
	Subscribers int
	Published   uint64
	Reconnects  uint64
	Rejected    uint64
	SlowDrops   uint64
}

type subscriber struct {
	profile string
	events  chan Event
}

// Hub owns event history and active streams.
type Hub struct {
	mu          sync.Mutex
	nextEvent   uint64
	nextClient  uint64
	published   uint64
	reconnects  uint64
	rejected    uint64
	slowDrops   uint64
	history     []scopedEvent
	subscribers map[uint64]subscriber
	now         func() time.Time
	heartbeat   time.Duration
	accessCheck time.Duration
}

// New creates an empty live event hub.
func New() *Hub {
	return &Hub{
		subscribers: make(map[uint64]subscriber),
		now:         time.Now,
		heartbeat:   15 * time.Second,
		accessCheck: time.Second,
	}
}

// Publish records and delivers an event within its optional profile scope.
func (hub *Hub) Publish(profile, eventType, resource string) {
	hub.mu.Lock()
	hub.nextEvent++
	hub.published++
	event := scopedEvent{Event: Event{ID: hub.nextEvent, Type: eventType, Resource: resource, At: hub.now().UTC()}, profile: profile}
	hub.history = append(hub.history, event)
	if len(hub.history) > historyLimit {
		hub.history = append([]scopedEvent(nil), hub.history[len(hub.history)-historyLimit:]...)
	}
	for id, subscriber := range hub.subscribers {
		if event.profile != "" && event.profile != subscriber.profile {
			continue
		}
		select {
		case subscriber.events <- event.Event:
		default:
			close(subscriber.events)
			delete(hub.subscribers, id)
			hub.slowDrops++
		}
	}
	hub.mu.Unlock()
}

func (hub *Hub) subscribe(profile string, after uint64) ([]Event, *subscription, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.subscribers) >= subscriberLimit {
		hub.rejected++
		return nil, nil, false
	}
	if after > 0 {
		hub.reconnects++
	}
	backlog := make([]Event, 0)
	for _, event := range hub.history {
		if event.ID > after && (event.profile == "" || event.profile == profile) {
			backlog = append(backlog, event.Event)
		}
	}
	hub.nextClient++
	events := make(chan Event, subscriberQueue)
	hub.subscribers[hub.nextClient] = subscriber{profile: profile, events: events}
	return backlog, &subscription{hub: hub, id: hub.nextClient, events: events}, true
}

// Metrics returns a consistent stream metrics snapshot.
func (hub *Hub) Metrics() Metrics {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return Metrics{
		Subscribers: len(hub.subscribers), Published: hub.published, Reconnects: hub.reconnects,
		Rejected: hub.rejected, SlowDrops: hub.slowDrops,
	}
}

func (subscription *subscription) close() {
	if subscription == nil || subscription.hub == nil {
		return
	}
	subscription.hub.mu.Lock()
	if current, found := subscription.hub.subscribers[subscription.id]; found {
		close(current.events)
		delete(subscription.hub.subscribers, subscription.id)
	}
	subscription.hub.mu.Unlock()
}
