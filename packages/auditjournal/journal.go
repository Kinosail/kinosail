// Package auditjournal owns Kinosail's durable HMAC-chained activity journal.
package auditjournal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultAuditRetention    time.Duration = 31_536_000_000_000_000 // 365 days.
	defaultPlaybackRetention time.Duration = 7_776_000_000_000_000  // 90 days.
	maxActivityEvents                      = 25_000
	auditNotificationQueue                 = 128
)

// Event is one durable activity record.
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

// Config supplies journal storage, retention, and notification policy.
type Config struct {
	DataDir           string
	AuditRetention    time.Duration
	PlaybackRetention time.Duration
	Notify            func(Event)
}

// Status is a safe journal health and queue snapshot.
type Status struct {
	Healthy            bool
	WriteFailures      uint64
	NotificationDrops  uint64
	NotificationQueued int
	LastError          string
}

// Journal stores and queries one installation's activity history.
type Journal struct {
	mu                sync.Mutex
	file              string
	events            []Event
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

// New validates and loads a journal. Load failures remain visible through Status.
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
	if config.Notify != nil && journal.writable {
		journal.notifications = make(chan Event, auditNotificationQueue)
		var done <-chan struct{}
		if ctx != nil {
			done = ctx.Done()
		}
		go journal.deliverNotifications(done)
	}
	return journal
}

// Record validates and durably appends one event.
func (journal *Journal) Record(event Event) {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if !journal.writable || !validEvent(event, false) {
		journal.failLocked("write activity journal", errors.New("activity event is invalid"))
		return
	}
	event = cloneEvent(event)
	previousEvents := append([]Event(nil), journal.events...)
	if len(journal.events) != 0 {
		event.Previous = journal.events[len(journal.events)-1].Integrity
	}
	event.Integrity = journal.sign(event)
	journal.events = append(journal.events, event)
	pruned := journal.pruneLocked(time.Now().UTC())
	var err error
	if journal.file != "" {
		if pruned {
			err = journal.rewriteLocked()
		} else {
			err = appendAuditEvent(journal.file, event)
		}
	}
	if err != nil {
		journal.events = previousEvents
		journal.failLocked("write activity journal", err)
		return
	}
	if journal.notify != nil && event.Category != "playback" {
		journal.enqueueNotification(event)
	}
}

// Query returns newest events first and clamps the result to 1 through 1000 items.
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

// RecentLogs returns the newest 25 events that are not playback events.
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

// TripwireWarning returns the newest public scanner warning.
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

// WriteJSONL writes all events in chronological order.
func (journal *Journal) WriteJSONL(writer io.Writer) error {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	encoder := json.NewEncoder(writer)
	for _, event := range journal.events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

// Status returns current persistence and notification health.
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

func validEvent(event Event, stored bool) bool {
	if !validCoreFields(event) || !validOptionalFields(event) || !validDetails(event.Details) {
		return false
	}
	if !stored && (event.Previous != "" || event.Integrity != "") {
		return false
	}
	data, err := json.Marshal(event)
	return err == nil && len(data) <= maxAuditEventBytes
}

func validCoreFields(event Event) bool {
	if !validRequiredString(event.ID, 128) || !validRequiredString(event.Time, 64) || !validRequiredString(event.Category, 64) || !validRequiredString(event.Action, 256) || !validRequiredString(event.Result, 64) {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, event.Time)
	return err == nil
}

func validOptionalFields(event Event) bool {
	for _, value := range []string{event.Actor, event.ActorID, event.Target, event.TargetID, event.RequestID, event.Remote} {
		if !validOptionalString(value, 4096) {
			return false
		}
	}
	return true
}

func validRequiredString(value string, limit int) bool {
	return value != "" && validOptionalString(value, limit)
}

func validOptionalString(value string, limit int) bool {
	return len(value) <= limit && !strings.ContainsRune(value, 0)
}

func validDetails(details map[string]string) bool {
	if len(details) > 128 {
		return false
	}
	for key, value := range details {
		if !validRequiredString(key, 256) || !validOptionalString(value, 4096) {
			return false
		}
	}
	return true
}
