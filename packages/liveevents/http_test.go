package liveevents

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type streamWriter struct {
	mu          sync.Mutex
	header      http.Header
	body        bytes.Buffer
	writes      int
	flushes     int
	failWriteAt int
	failFlushAt int
}

func newStreamWriter() *streamWriter {
	return &streamWriter{header: make(http.Header)}
}

func (writer *streamWriter) Header() http.Header { return writer.header }
func (*streamWriter) WriteHeader(int)            {}

func (writer *streamWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.writes++
	if writer.writes == writer.failWriteAt {
		return 0, errors.New("write failed")
	}
	return writer.body.Write(data)
}

func (writer *streamWriter) FlushError() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.flushes++
	if writer.flushes == writer.failFlushAt {
		return errors.New("flush failed")
	}
	return nil
}

func (writer *streamWriter) snapshot() (string, int) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.body.String(), writer.flushes
}

func testAccess(allowed func(*http.Request) bool) Access {
	return Access{
		Profile: func(*http.Request) string { return "viewer" },
		Allowed: allowed,
		Error: func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			http.Error(writer, err.Error(), status)
		},
	}
}

func waitForStream(t *testing.T, writer *streamWriter, predicate func(string, int) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		body, flushes := writer.snapshot()
		if predicate(body, flushes) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	body, flushes := writer.snapshot()
	t.Fatalf("stream did not reach state: body=%q flushes=%d", body, flushes)
}

func TestHandlerRejectsInvalidAndExcessStreams(t *testing.T) {
	hub := New()
	mux := http.NewServeMux()
	hub.Register(mux, testAccess(func(*http.Request) bool { return true }))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
	request.Header.Set("Last-Event-ID", "not-a-number")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || len(hub.subscribers) != 0 {
		t.Fatalf("invalid cursor = %d, subscribers=%d", response.Code, len(hub.subscribers))
	}
	for id := 0; id < subscriberLimit; id++ {
		_, _, _ = hub.subscribe("viewer", 0)
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
	response = httptest.NewRecorder()
	hub.Handler(testAccess(func(*http.Request) bool { return true })).ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("excess stream = %d %q", response.Code, response.Body.String())
	}
}

func TestHandlerStreamsBacklogAndPublishedEvents(t *testing.T) {
	hub := New()
	hub.heartbeat = time.Hour
	hub.accessCheck = time.Hour
	hub.Publish("viewer", "library.updated", "/api/v1/library")
	ctx, cancel := context.WithCancel(t.Context())
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/events", nil)
	writer := newStreamWriter()
	done := make(chan struct{})
	go func() {
		hub.Handler(testAccess(func(*http.Request) bool { return true })).ServeHTTP(writer, request)
		close(done)
	}()
	waitForStream(t, writer, func(body string, flushes int) bool {
		return strings.Contains(body, "library.updated") && flushes == 1
	})
	hub.Publish("viewer", "download.updated", "/api/v1/downloads/id")
	waitForStream(t, writer, func(body string, flushes int) bool {
		return strings.Contains(body, "download.updated") && flushes == 2
	})
	cancel()
	<-done
	if writer.Header().Get("Content-Type") != "text/event-stream" || writer.Header().Get("Cache-Control") != "no-store" || writer.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("stream headers = %v", writer.Header())
	}
}

func TestHandlerStopsForClosedStreamAndWriteFailures(t *testing.T) {
	for name, test := range map[string]struct {
		prepare func(*Hub, *streamWriter)
		trigger func(*Hub)
	}{
		"closed": {
			trigger: func(hub *Hub) {
				hub.mu.Lock()
				for id, subscriber := range hub.subscribers {
					delete(hub.subscribers, id)
					close(subscriber.events)
				}
				hub.mu.Unlock()
			},
		},
		"event write": {
			prepare: func(_ *Hub, writer *streamWriter) { writer.failWriteAt = 2 },
			trigger: func(hub *Hub) { hub.Publish("viewer", "download.updated", "/api/v1/downloads/id") },
		},
		"event flush": {
			prepare: func(_ *Hub, writer *streamWriter) { writer.failFlushAt = 2 },
			trigger: func(hub *Hub) { hub.Publish("viewer", "download.updated", "/api/v1/downloads/id") },
		},
	} {
		t.Run(name, func(t *testing.T) {
			hub := New()
			hub.heartbeat, hub.accessCheck = time.Hour, time.Hour
			writer := newStreamWriter()
			if test.prepare != nil {
				test.prepare(hub, writer)
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
			done := make(chan struct{})
			go func() {
				hub.Handler(testAccess(func(*http.Request) bool { return true })).ServeHTTP(writer, request)
				close(done)
			}()
			waitForStream(t, writer, func(_ string, flushes int) bool { return flushes == 1 })
			test.trigger(hub)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("stream did not stop")
			}
		})
	}
}

func TestHandlerStopsWhenBacklogWriteFails(t *testing.T) {
	hub := New()
	hub.Publish("viewer", "library.updated", "/api/v1/library")
	writer := newStreamWriter()
	writer.failWriteAt = 2
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
	hub.Handler(testAccess(func(*http.Request) bool { return true })).ServeHTTP(writer, request)
	if hub.Metrics().Subscribers != 0 {
		t.Fatal("failed backlog stream remained subscribed")
	}
}

func TestHandlerStopsForFlushHeartbeatAndAccessResults(t *testing.T) {
	for name, test := range map[string]struct {
		heartbeat, accessCheck time.Duration
		failFlushAt            int
		allowed                bool
	}{
		"initial flush": {time.Hour, time.Hour, 1, true},
		"heartbeat":     {time.Millisecond, time.Hour, 2, true},
		"access denied": {time.Hour, time.Millisecond, 0, false},
	} {
		t.Run(name, func(t *testing.T) {
			hub := New()
			hub.heartbeat, hub.accessCheck = test.heartbeat, test.accessCheck
			writer := newStreamWriter()
			writer.failFlushAt = test.failFlushAt
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
			done := make(chan struct{})
			go func() {
				hub.Handler(testAccess(func(*http.Request) bool { return test.allowed })).ServeHTTP(writer, request)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("stream did not stop")
			}
			if name == "heartbeat" {
				body, _ := writer.snapshot()
				if !strings.Contains(body, ": keepalive") {
					t.Fatalf("heartbeat output = %q", body)
				}
			}
		})
	}
}

func TestLastEventIDAndWireFormatAreBounded(t *testing.T) {
	for value, valid := range map[string]bool{"": true, "0": true, "18446744073709551615": true, "-1": false, "one": false, "184467440737095516150": false} {
		if _, err := lastEventID(value); (err == nil) != valid {
			t.Errorf("lastEventID(%q) validity = %v, want %v", value, err == nil, valid)
		}
	}
	output := httptest.NewRecorder()
	event := Event{ID: 7, Type: "library.updated", Resource: "/api/v1/library"}
	if err := writeEvent(output, event); err != nil || !strings.Contains(output.Body.String(), "id: 7\n") {
		t.Fatalf("event wire format = %q, %v", output.Body.String(), err)
	}
	if err := writeEvent(output, Event{At: time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC)}); err == nil {
		t.Fatal("invalid event time was serialized")
	}
	failing := newStreamWriter()
	failing.failWriteAt = 1
	if err := writeEvent(failing, event); err == nil {
		t.Fatal("event write failure was ignored")
	}
}
