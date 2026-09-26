package auditjournal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Actor struct{ Name, ID string }

type HTTPConfig struct {
	Action        func(*http.Request) (string, string)
	Details       func(*http.Request) map[string]string
	Snapshot      func(*http.Request) map[string]string
	Actor         func(*http.Request) Actor
	FallbackActor func(*http.Request) Actor
	Target        func(*http.Request) string
	MarkSecurity  func(*http.Request)
	RequestID     func(*http.Request) string
	Remote        func(*http.Request) string
	NewID         func() string
	Now           func() time.Time
}

type HTTPTracker struct {
	journal  *Journal
	config   HTTPConfig
	mu       sync.RWMutex
	title    func(string) string
	snapshot func(*http.Request) map[string]string
}

type ResponseWriter struct {
	http.ResponseWriter
	Status int
	Bytes  int64
}

func (writer *ResponseWriter) WriteHeader(status int) {
	if writer.Status != 0 {
		return
	}
	writer.Status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *ResponseWriter) Write(data []byte) (int, error) {
	if writer.Status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(data)
	writer.Bytes += int64(written)
	return written, err
}

// ReadFrom preserves the underlying writer's file transfer fast path.
func (writer *ResponseWriter) ReadFrom(source io.Reader) (int64, error) {
	if writer.Status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	var written int64
	var err error
	if readerFrom, ok := writer.ResponseWriter.(io.ReaderFrom); ok {
		written, err = readerFrom.ReadFrom(source)
	} else {
		written, err = io.Copy(writer.ResponseWriter, source)
	}
	writer.Bytes += written
	return written, err
}

func (writer *ResponseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func NewHTTPTracker(journal *Journal, config HTTPConfig) *HTTPTracker {
	if config.Now == nil {
		config.Now = time.Now
	}
	return &HTTPTracker{journal: journal, config: config, snapshot: config.Snapshot}
}

func (tracker *HTTPTracker) SetTitle(title func(string) string) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	tracker.title = title
	tracker.mu.Unlock()
}

func (tracker *HTTPTracker) SetSnapshot(snapshot func(*http.Request) map[string]string) {
	if tracker == nil {
		return
	}
	tracker.mu.Lock()
	tracker.snapshot = snapshot
	tracker.mu.Unlock()
}

func (tracker *HTTPTracker) ResolveTitle(id string) string {
	if tracker == nil {
		return ""
	}
	tracker.mu.RLock()
	title := tracker.title
	tracker.mu.RUnlock()
	if title == nil {
		return ""
	}
	return title(id)
}

func (tracker *HTTPTracker) Track(next http.Handler, writer http.ResponseWriter, request *http.Request) { //nolint:cyclop,gocognit,funlen // Audit enrichment keeps request outcomes explicit.
	if !tracker.valid() || next == nil || writer == nil || request == nil {
		return
	}
	action, category := tracker.config.Action(request)
	if action == "" {
		next.ServeHTTP(writer, request)
		return
	}
	details := tracker.config.Details(request)
	if details == nil {
		details = make(map[string]string)
	}
	tracker.mu.RLock()
	snapshot := tracker.snapshot
	tracker.mu.RUnlock()
	before := map[string]string(nil)
	if snapshot != nil {
		before = snapshot(request)
	}
	capture := &ResponseWriter{ResponseWriter: writer}
	next.ServeHTTP(capture, request)
	if snapshot != nil {
		for key, value := range before {
			details["before."+key] = value
		}
		for key, value := range snapshot(request) {
			details["after."+key] = value
		}
	}
	status := capture.Status
	if status == 0 {
		status = http.StatusOK
	}
	details["status"] = strconv.Itoa(status)
	result := "success"
	if status >= http.StatusBadRequest {
		result = "failure"
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		details["attemptedAction"] = action
		action, category, result = "access.denied", "security", "denied"
		tracker.config.MarkSecurity(request)
	}
	actor := tracker.config.Actor(request)
	if actor.ID == "" {
		actor = tracker.config.FallbackActor(request)
	}
	target, targetID := tracker.config.Target(request), request.PathValue("id")
	if category == "playback" {
		if title := tracker.ResolveTitle(targetID); title != "" {
			target, details["title"] = title, title
		}
	}
	event := tracker.Event(request, category, action, target, targetID, result, details)
	event.Actor, event.ActorID = actor.Name, actor.ID
	if name := ActorName(request.Context()); name != "" {
		event.Actor = name
	}
	if event.Actor == "" && strings.HasPrefix(action, "session.") {
		event.Actor = strings.TrimSpace(request.FormValue("name"))
	}
	tracker.Record(event)
}

func (tracker *HTTPTracker) Denied(request *http.Request, reason string) {
	if !tracker.validRequest(request) {
		return
	}
	tracker.config.MarkSecurity(request)
	details := map[string]string{"reason": reason, "method": request.Method, "path": request.URL.Path}
	tracker.Record(tracker.Event(request, "security", "access.denied", request.URL.Path, "", "denied", details))
}

func (tracker *HTTPTracker) Tripwire(request *http.Request, reason string, blocked bool) {
	if !tracker.validRequest(request) {
		return
	}
	tracker.config.MarkSecurity(request)
	details := map[string]string{"reason": reason, "method": request.Method, "path": request.URL.Path, "quarantined": strconv.FormatBool(blocked)}
	tracker.Record(tracker.Event(request, "security", "security.tripwire", request.URL.Path, "", "denied", details))
}

func (tracker *HTTPTracker) PasskeyRisk(request *http.Request, actor Actor, credentialID []byte, signCount uint32, backupEligible, backedUp bool) {
	if !tracker.validRequest(request) {
		return
	}
	id := sha256.Sum256(credentialID)
	details := map[string]string{"signal": "signature counter anomaly", "signCount": strconv.FormatUint(uint64(signCount), 10), "backupEligible": strconv.FormatBool(backupEligible), "backedUp": strconv.FormatBool(backedUp)}
	event := tracker.Event(request, "security", "passkey.risk", fmt.Sprintf("passkey %x", id[:4]), "", "warning", details)
	event.Actor, event.ActorID = actor.Name, actor.ID
	tracker.Record(event)
}

func (tracker *HTTPTracker) Playback(request *http.Request, action, id, title string, seconds float64, watched bool) {
	if !tracker.validRequest(request) {
		return
	}
	details := map[string]string{"title": title, "seconds": strconv.FormatFloat(seconds, 'f', -1, 64)}
	if watched {
		details["watched"] = "true"
	}
	event := tracker.Event(request, "playback", action, title, id, "success", details)
	actor := tracker.config.Actor(request)
	event.Actor, event.ActorID = actor.Name, actor.ID
	tracker.Record(event)
}

func (tracker *HTTPTracker) Event(request *http.Request, category, action, target, targetID, result string, details map[string]string) Event {
	if !tracker.validRequest(request) {
		return Event{}
	}
	return Event{ID: tracker.config.NewID(), Time: tracker.config.Now().UTC().Format(time.RFC3339Nano), Category: category, Action: action, Target: target, TargetID: targetID, Result: result, RequestID: tracker.config.RequestID(request), Remote: tracker.config.Remote(request), Details: details}
}

func (tracker *HTTPTracker) Record(event Event) {
	if tracker != nil && tracker.journal != nil {
		tracker.journal.Record(event)
	}
}

func (tracker *HTTPTracker) validRequest(request *http.Request) bool {
	return request != nil && tracker.valid()
}

func (tracker *HTTPTracker) valid() bool { //nolint:cyclop // Every required dependency must be present.
	if tracker == nil || tracker.journal == nil {
		return false
	}
	config := tracker.config
	for _, configured := range []bool{
		config.Action != nil, config.Details != nil, config.Actor != nil, config.FallbackActor != nil, config.Target != nil,
		config.MarkSecurity != nil, config.RequestID != nil, config.Remote != nil, config.NewID != nil, config.Now != nil,
	} {
		if !configured {
			return false
		}
	}
	return true
}

type actorContextKey struct{}

func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorName(ctx context.Context) string {
	actor, _ := ctx.Value(actorContextKey{}).(string)
	return actor
}
