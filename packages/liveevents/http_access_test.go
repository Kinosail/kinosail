package liveevents

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandlerRechecksAccessWhenAccessChanges(t *testing.T) {
	hub := New()
	hub.accessCheck = time.Hour
	var allowed atomic.Bool
	allowed.Store(true)
	var signal sync.Mutex
	changed := make(chan struct{})
	access := testAccess(func(*http.Request) bool { return allowed.Load() })
	access.Changed = func() <-chan struct{} {
		signal.Lock()
		defer signal.Unlock()
		return changed
	}
	writer := newStreamWriter()
	done := make(chan struct{})
	go func() {
		hub.Handler(access).ServeHTTP(writer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil))
		close(done)
	}()
	waitForStream(t, writer, func(_ string, flushes int) bool { return flushes > 0 })
	signal.Lock()
	close(changed)
	changed = make(chan struct{})
	signal.Unlock()
	hub.Publish("", "library.updated", "/api/v1/library")
	waitForStream(t, writer, func(body string, _ int) bool { return strings.Contains(body, "library.updated") })
	allowed.Store(false)
	signal.Lock()
	close(changed)
	signal.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("revoked stream stayed open until the periodic check")
	}
}

func TestStalledStreamIsReleasedByItsWriteDeadline(t *testing.T) {
	hub := New()
	hub.heartbeat, hub.writeTimeout = time.Hour, 100*time.Millisecond
	done := make(chan struct{})
	web := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer close(done)
		hub.Handler(testAccess(func(*http.Request) bool { return true })).ServeHTTP(writer, request)
	}))
	t.Cleanup(web.Close)
	connection, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", web.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if _, err := connection.Write([]byte("GET /api/v1/events HTTP/1.1\r\nHost: kinosail.test\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for hub.Metrics().Subscribers == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	resource := strings.Repeat("x", 1<<20)
	for range subscriberQueue {
		hub.Publish("", "library.updated", resource)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a client that stopped reading held its stream handler")
	}
}
