package liveevents

import (
	"sync"
	"time"
)

const (
	historyLimit           = 128
	subscriberLimit        = 256
	profileSubscriberLimit = 16
	subscriberQueue        = 16
)

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

type Hub struct {
	mu           sync.Mutex
	nextEvent    uint64
	nextClient   uint64
	published    uint64
	reconnects   uint64
	rejected     uint64
	slowDrops    uint64
	history      [historyLimit]scopedEvent
	historySize  int
	subscribers  map[uint64]subscriber
	profiles     map[string]int
	now          func() time.Time
	heartbeat    time.Duration
	accessCheck  time.Duration
	writeTimeout time.Duration
	observer     func(string, string, string)
}

func New() *Hub {
	return &Hub{
		subscribers:  make(map[uint64]subscriber),
		profiles:     make(map[string]int),
		now:          time.Now,
		heartbeat:    15 * time.Second,
		accessCheck:  15 * time.Second,
		writeTimeout: 10 * time.Second,
	}
}

func (hub *Hub) Publish(profile, eventType, resource string) {
	hub.mu.Lock()
	hub.nextEvent++
	hub.published++
	event := scopedEvent{Event: Event{ID: hub.nextEvent, Type: eventType, Resource: resource, At: hub.now().UTC()}, profile: profile}
	hub.history[(event.ID-1)%historyLimit] = event
	hub.historySize = min(hub.historySize+1, historyLimit)
	for id, subscriber := range hub.subscribers {
		if event.profile != "" && event.profile != subscriber.profile {
			continue
		}
		select {
		case subscriber.events <- event.Event:
		default:
			hub.removeLocked(id)
			hub.slowDrops++
		}
	}
	observer := hub.observer
	hub.mu.Unlock()
	if observer != nil {
		observer(profile, eventType, resource)
	}
}

func (hub *Hub) SetObserver(observer func(string, string, string)) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.observer = observer
}

func (hub *Hub) subscribe(profile string, after uint64) ([]Event, *subscription, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.subscribers) >= subscriberLimit || hub.profiles[profile] >= profileSubscriberLimit {
		hub.rejected++
		return nil, nil, false
	}
	if after > 0 {
		hub.reconnects++
	}
	backlog := make([]Event, 0)
	for index := range hub.historySize {
		event := hub.history[(hub.nextEvent-uint64(hub.historySize)+uint64(index))%historyLimit]
		if event.ID > after && (event.profile == "" || event.profile == profile) {
			backlog = append(backlog, event.Event)
		}
	}
	hub.nextClient++
	events := make(chan Event, subscriberQueue)
	hub.subscribers[hub.nextClient] = subscriber{profile: profile, events: events}
	hub.profiles[profile]++
	return backlog, &subscription{hub: hub, id: hub.nextClient, events: events}, true
}

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
	subscription.hub.removeLocked(subscription.id)
	subscription.hub.mu.Unlock()
}

func (hub *Hub) removeLocked(id uint64) {
	current, found := hub.subscribers[id]
	if !found {
		return
	}
	close(current.events)
	delete(hub.subscribers, id)
	if hub.profiles[current.profile]--; hub.profiles[current.profile] <= 0 {
		delete(hub.profiles, current.profile)
	}
}
