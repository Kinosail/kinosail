package auditjournal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultAuditRetention       time.Duration = 31_536_000_000_000_000
	defaultPlaybackRetention    time.Duration = 7_776_000_000_000_000
	maxActivityEvents                         = 25_000
	compactedActivityEvents                   = 22_500
	compactedJournalBytes                     = maxAuditJournalBytes - 8<<20
	retentionCompactionInterval               = time.Hour
	auditNotificationQueue                    = 128
)

type Event struct {
	ID        string            `json:"id"`
	Time      string            `json:"time"`
	Category  string            `json:"category"`
	Action    string            `json:"action"`
	Actor     string            `json:"actor,omitempty"`
	ActorID   string            `json:"actorId,omitempty"`
	Target    string            `json:"target,omitempty"`
	TargetID  string            `json:"targetId,omitempty"`
	Result    string            `json:"result"`
	RequestID string            `json:"requestId,omitempty"`
	Remote    string            `json:"remote,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	Previous  string            `json:"previous,omitempty"`
	Integrity string            `json:"integrity,omitempty"`
}

type Config struct {
	DataDir           string
	AuditRetention    time.Duration
	PlaybackRetention time.Duration
	Notify            func(Event)
}

type Status struct {
	Healthy            bool
	WriteFailures      uint64
	NotificationDrops  uint64
	NotificationQueued int
	LastError          string
}

type eventMeta struct {
	created time.Time
	size    int64
}

type Journal struct {
	mu                sync.Mutex
	writeMu           sync.Mutex
	file              string
	events            []Event
	meta              []eventMeta
	bytes             int64
	nextExpiry        time.Time
	compacted         time.Time
	flood             floodGuard
	notify            func(Event)
	auditRetention    time.Duration
	playbackRetention time.Duration
	writeFailures     atomic.Uint64
	notificationDrops atomic.Uint64
	lastErr           string
	key               []byte
	notifications     chan Event
	writable          bool
}

func New(ctx context.Context, config Config) *Journal {
	journal := &Journal{
		notify:            config.Notify,
		auditRetention:    config.AuditRetention,
		playbackRetention: config.PlaybackRetention,
	}
	if journal.auditRetention <= 0 {
		journal.auditRetention = defaultAuditRetention
	}
	if journal.playbackRetention <= 0 {
		journal.playbackRetention = defaultPlaybackRetention
	}
	if err := journal.load(config.DataDir); err != nil {
		journal.failLocked("load activity journal", err)
	}
	var done <-chan struct{}
	if ctx != nil {
		done = ctx.Done()
	}
	if config.Notify != nil && journal.writable {
		journal.notifications = make(chan Event, auditNotificationQueue)
		go journal.deliverNotifications(done)
	}
	if ctx != nil && journal.writable {
		go journal.summarizeSecurityEvents(done)
	}
	return journal
}

func (journal *Journal) Record(event Event) {
	journal.store(journal.admit(event))
}

func (journal *Journal) store(events []admittedEvent) {
	if len(events) == 0 {
		return
	}
	journal.writeMu.Lock()
	defer journal.writeMu.Unlock()
	for _, admitted := range events {
		event, err := journal.persist(admitted.event, admitted.created, time.Now().UTC())
		if err != nil {
			journal.mu.Lock()
			journal.failLocked("write activity journal", err)
			journal.mu.Unlock()
			return
		}
		if journal.notify != nil && event.Category != "playback" {
			journal.enqueueNotification(event)
		}
	}
}

func (journal *Journal) admit(event Event) []admittedEvent {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if !journal.writable || !validEvent(event, false) {
		journal.failLocked("write activity journal", errors.New("activity event is invalid"))
		return nil
	}
	event = cloneEvent(event)
	created, _ := time.Parse(time.RFC3339Nano, event.Time)
	admitted, summaries := journal.flood.admit(&event, created)
	events := summarizedEvents(summaries, created)
	if !admitted {
		return events
	}
	if _, summarized := event.Details[suppressedDetail]; summarized && !validEvent(event, false) {
		delete(event.Details, suppressedDetail)
	}
	return append(events, admittedEvent{event, created})
}

func (journal *Journal) Query(category string, limit int) []Event {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if limit < 1 {
		limit = 1
	} else if limit > 1000 {
		limit = 1000
	}
	result := make([]Event, 0, min(limit, len(journal.events)))
	for index := len(journal.events) - 1; index >= 0 && len(result) < limit; index-- {
		if category == "" || journal.events[index].Category == category {
			result = append(result, cloneEvent(journal.events[index]))
		}
	}
	return result
}

func (journal *Journal) RecentLogs() []Event {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	result := make([]Event, 0, min(25, len(journal.events)))
	for index := len(journal.events) - 1; index >= 0 && len(result) < 25; index-- {
		if journal.events[index].Category != "playback" {
			result = append(result, cloneEvent(journal.events[index]))
		}
	}
	return result
}

func (journal *Journal) TripwireWarning() string {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	for index := len(journal.events) - 1; index >= 0; index-- {
		if journal.events[index].Action == "security.tripwire" {
			if journal.events[index].Details["quarantined"] == "true" {
				return "Public scanner quarantined; local and WireGuard access remain available."
			}
			return "Public scanner probe detected; local and WireGuard access remain available."
		}
	}
	return ""
}

func (journal *Journal) WriteJSONL(writer io.Writer) error {
	journal.mu.Lock()
	events := journal.events
	journal.mu.Unlock()
	encoder := json.NewEncoder(writer)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func (journal *Journal) Status() Status {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	failures := journal.writeFailures.Load()
	queued := 0
	if journal.notifications != nil {
		queued = len(journal.notifications)
	}
	return Status{failures == 0, failures, journal.notificationDrops.Load(), queued, journal.lastErr}
}

func (journal *Journal) failLocked(message string, err error) {
	journal.writeFailures.Add(1)
	journal.lastErr = err.Error()
	slog.Error(message, "error", err)
}

func cloneEvent(event Event) Event {
	if event.Details != nil {
		details := make(map[string]string, len(event.Details))
		for key, value := range event.Details {
			details[key] = value
		}
		event.Details = details
	}
	return event
}
