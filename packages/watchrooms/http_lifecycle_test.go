package watchrooms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestHTTPConnectionClosesWhenRoomExpires(t *testing.T) {
	harness := newHTTPHarness(t)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	roomID, _ := harness.rooms.Create("leader", "movie", 0)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	connection := dialHTTPRoom(t, server.URL, roomID, "leader", "")
	readHTTPEvent(t, connection)

	harness.rooms.mu.Lock()
	harness.rooms.rooms[roomID].updated = time.Now().Add(-2 * time.Hour)
	harness.rooms.mu.Unlock()
	if err := wsjson.Write(t.Context(), connection, Event{Action: "play", Media: "movie"}); err != nil {
		t.Fatal(err)
	}
	assertHTTPConnectionClosed(t, connection)
}

func TestHTTPAccessMonitorClosesInvisibleRoom(t *testing.T) {
	harness := newHTTPHarness(t)
	var visible atomic.Bool
	visible.Store(true)
	config := httpConfig(harness.rooms, harness.index)
	config.CanView = func(*http.Request, library.Item) bool { return visible.Load() }
	harness.handler = MustNewHTTP(config)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	roomID, _ := harness.rooms.Create("leader", "movie", 0)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	connection := dialHTTPRoom(t, server.URL, roomID, "leader", "")
	readHTTPEvent(t, connection)
	visible.Store(false)
	assertHTTPConnectionClosed(t, connection)
}

func TestWriteEventsClosesSubscriptionAfterWriteFailure(t *testing.T) {
	client, server := httpWebSocketPair(t)
	rooms := New(time.Hour)
	roomID, _ := rooms.Create("leader", "movie", 0)
	_, subscription, _ := rooms.Join(roomID, "leader")
	if err := server.CloseNow(); err != nil {
		t.Fatal(err)
	}
	if !rooms.Update(subscription, Event{Action: "play", Media: "movie"}, true) {
		t.Fatal("event was not queued")
	}
	events := subscription.Events()
	newHTTPHarness(t).handler.writeEvents(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), server, subscription)
	if _, open := <-events; open {
		t.Fatal("subscription remained open after write failure")
	}
	_ = client.CloseNow()
}

func httpWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		accepted <- connection
		<-release
	}))
	client, response, err := websocket.Dial(t.Context(), "ws"+server.URL[len("http"):], nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	peer := <-accepted
	t.Cleanup(func() {
		close(release)
		_ = peer.CloseNow()
		_ = client.CloseNow()
		server.Close()
	})
	return client, peer
}

func assertHTTPConnectionClosed(t *testing.T, connection *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := wsjson.Read(ctx, connection, &Event{}); err == nil {
		t.Fatal("connection remained open")
	}
	_ = connection.CloseNow()
}

func TestEventDeliveryReauthorizesBeforeSendingHiddenMedia(t *testing.T) {
	harness := newHTTPHarness(t)
	client, server := httpWebSocketPair(t)
	roomID, _ := harness.rooms.Create("leader", "movie", 0)
	_, leader, _ := harness.rooms.Join(roomID, "leader")
	defer leader.Close()
	_, restricted, _ := harness.rooms.Join(roomID, "restricted")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Viewer", "restricted")
	if !harness.rooms.Update(leader, Event{Action: "media", Media: "hidden"}, true) {
		t.Fatal("leader update rejected")
	}
	done := make(chan struct{})
	go func() { defer close(done); harness.handler.writeEvents(request, server, restricted) }()
	assertHTTPConnectionClosed(t, client)
	<-done
}
