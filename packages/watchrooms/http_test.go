package watchrooms

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type httpIndexStub struct {
	items []library.Item
	err   error
}

func (index *httpIndexStub) Find(id string) (library.Item, bool) {
	for _, item := range index.items {
		if item.ID == id {
			return item, true
		}
	}
	return library.Item{}, false
}

func (*httpIndexStub) Safe(path string) bool { return path != "unsafe" }

func (index *httpIndexStub) Snapshot() ([]library.Item, error) {
	return append([]library.Item(nil), index.items...), index.err
}

type httpHarness struct {
	handler *HTTP
	rooms   *Rooms
	index   *httpIndexStub
}

func newHTTPHarness(t *testing.T) httpHarness {
	t.Helper()
	rooms := New(time.Hour)
	index := &httpIndexStub{items: []library.Item{
		{ID: "movie", Kind: "video", Path: "movie"},
		{ID: "song", Kind: "audio", Path: "song"},
		{ID: "photo", Kind: "photo", Path: "photo"},
		{ID: "hidden", Kind: "video", Path: "hidden", Library: "hidden"},
		{ID: "unsafe", Kind: "video", Path: "unsafe"},
	}}
	handler := MustNewHTTP(httpConfig(rooms, index))
	return httpHarness{handler: handler, rooms: rooms, index: index}
}

func httpConfig(rooms *Rooms, index *httpIndexStub) HTTPConfig {
	return HTTPConfig{
		Rooms: rooms, Index: index,
		ViewerID: func(request *http.Request) string { return request.Header.Get("X-Viewer") },
		CanView:  func(_ *http.Request, item library.Item) bool { return item.Library != "hidden" },
		Reauthorize: func(request *http.Request, _ string) (*http.Request, string, bool) {
			return request, request.Header.Get("X-Viewer"), request.Header.Get("X-Deny") == ""
		},
		Failure: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			http.Error(writer, message, status)
		},
		NotFound: func(writer http.ResponseWriter, request *http.Request) { http.NotFound(writer, request) },
	}
}

func TestHTTPConfigurationRejectsMissingAdapters(t *testing.T) {
	if _, err := NewHTTP(HTTPConfig{}); err == nil {
		t.Fatal("empty HTTP configuration was accepted")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustNewHTTP accepted empty configuration")
		}
	}()
	MustNewHTTP(HTTPConfig{})
}

func TestHTTPRoutesValidateCreateRedirectAndPlayerProjection(t *testing.T) { //nolint:cyclop // One lifecycle protects create, redirect, and projection behavior.
	harness := newHTTPHarness(t)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	for name, form := range map[string]url.Values{
		"missing":   {},
		"photo":     {"media": {"photo"}},
		"unsafe":    {"media": {"unsafe"}},
		"hidden":    {"media": {"hidden"}},
		"malformed": {"media": {"movie"}, "seconds": {"later"}},
		"negative":  {"media": {"movie"}, "seconds": {"-1"}},
		"nan":       {"media": {"movie"}, "seconds": {"NaN"}},
		"infinite":  {"media": {"movie"}, "seconds": {"+Inf"}},
		"oversized": {"media": {"movie"}, "seconds": {strconv.FormatFloat(MaximumSeconds+1, 'f', -1, 64)}},
	} {
		t.Run(name, func(t *testing.T) {
			response := serveRoomRequest(mux, http.MethodPost, "/watch-together", form.Encode(), "leader", "")
			if response.Code != http.StatusBadRequest || len(harness.rooms.rooms) != 0 {
				t.Fatalf("invalid create = %d %q", response.Code, response.Body.String())
			}
		})
	}
	created := serveRoomRequest(mux, http.MethodPost, "/watch-together", "media=movie", "leader", "")
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create = %d %q", created.Code, created.Body.String())
	}
	location, err := url.Parse(created.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	roomID := location.Query().Get("room")
	redirect := serveRoomRequest(mux, http.MethodGet, "/room/"+roomID, "", "leader", "")
	if redirect.Code != http.StatusSeeOther || redirect.Header().Get("Location") != created.Header().Get("Location") {
		t.Fatalf("redirect = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}
	if missing := serveRoomRequest(mux, http.MethodGet, "/room/missing", "", "leader", ""); missing.Code != http.StatusNotFound {
		t.Fatalf("missing room = %d", missing.Code)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/movie?room="+roomID, nil)
	request.Header.Set("X-Viewer", "leader")
	harness.index.err = errors.New("snapshot failed")
	id, leader, items := harness.handler.Player(request, harness.index.items[0])
	if id != roomID || !leader || len(items) != 3 {
		t.Fatalf("player = %q, %v, %#v", id, leader, items)
	}
	if id, _, _ = harness.handler.Player(request, harness.index.items[1]); id != "" {
		t.Fatalf("mismatched player room = %q", id)
	}
}

func TestHTTPCreateReportsRoomCapacity(t *testing.T) {
	harness := newHTTPHarness(t)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	now := time.Now()
	for index := range maxRooms {
		harness.rooms.rooms[strconv.Itoa(index)] = &room{updated: now}
	}
	response := serveRoomRequest(mux, http.MethodPost, "/watch-together", "media=movie", "leader", "")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("capacity response = %d %q", response.Code, response.Body.String())
	}
}

func TestHTTPWebSocketPreservesLeaderAndValidationRules(t *testing.T) { //nolint:cyclop // One live socket lifecycle protects leader and viewer boundaries.
	harness := newHTTPHarness(t)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	roomID, ok := harness.rooms.Create("leader", "movie", 0)
	if !ok {
		t.Fatal("room was not created")
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	leader := dialHTTPRoom(t, server.URL, roomID, "leader", "")
	defer func() { _ = leader.CloseNow() }()
	if event := readHTTPEvent(t, leader); !event.Leader || event.Media != "movie" {
		t.Fatalf("leader state = %#v", event)
	}
	viewer := dialHTTPRoom(t, server.URL, roomID, "viewer", "")
	defer func() { _ = viewer.CloseNow() }()
	readHTTPEvent(t, viewer)
	if err := wsjson.Write(t.Context(), viewer, Event{Action: "play", Media: "movie", Seconds: 2}); err != nil {
		t.Fatal(err)
	}
	if denied := readHTTPEvent(t, viewer); denied.Action != "denied" {
		t.Fatalf("viewer event = %#v", denied)
	}
	for _, event := range []Event{{Action: "play", Media: "hidden", Seconds: 2}, {Action: "play", Media: "movie", Seconds: 3}} {
		if err := wsjson.Write(t.Context(), leader, event); err != nil {
			t.Fatal(err)
		}
	}
	if event := readHTTPEvent(t, leader); event.Seconds != 3 || event.Action != "play" {
		t.Fatalf("leader event = %#v", event)
	}
	if err := wsjson.Write(t.Context(), leader, Event{Action: "observe", Drift: 1.25}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Write(t.Context(), leader, Event{Action: "observe", Drift: -1}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := wsjson.Read(ctx, leader, &Event{}); err == nil {
		t.Fatal("invalid drift retained connection")
	}
}

func TestHTTPJoinAndAccessFailuresCloseConnections(t *testing.T) {
	harness := newHTTPHarness(t)
	mux := http.NewServeMux()
	harness.handler.Register(mux)
	roomID, _ := harness.rooms.Create("leader", "movie", 0)
	if response := serveRoomRequest(mux, http.MethodGet, "/api/v1/watch-rooms/missing/events", "", "leader", ""); response.Code != http.StatusNotFound {
		t.Fatalf("missing socket room = %d", response.Code)
	}
	if response := serveRoomRequest(mux, http.MethodGet, "/api/v1/watch-rooms/"+roomID+"/events", "", "leader", ""); response.Code == http.StatusSwitchingProtocols {
		t.Fatal("plain HTTP request upgraded")
	}
	accessServer := httptest.NewServer(mux)
	t.Cleanup(accessServer.Close)
	denied := dialHTTPRoom(t, accessServer.URL, roomID, "leader", "true")
	readHTTPEvent(t, denied)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := wsjson.Read(ctx, denied, &Event{}); err == nil {
		t.Fatal("revoked viewer retained connection")
	}
	_ = denied.CloseNow()

	var views atomic.Int64
	config := httpConfig(harness.rooms, harness.index)
	config.CanView = func(*http.Request, library.Item) bool { return views.Add(1) == 1 }
	handler := MustNewHTTP(config)
	mux = http.NewServeMux()
	handler.Register(mux)
	visibilityServer := httptest.NewServer(mux)
	t.Cleanup(visibilityServer.Close)
	closed := dialHTTPRoom(t, visibilityServer.URL, roomID, "leader", "")
	if err := wsjson.Read(t.Context(), closed, &Event{}); err == nil {
		t.Fatal("visibility loss retained connection")
	}
	_ = closed.CloseNow()
}

func TestHTTPEventValidationRejectsUntrustedValues(t *testing.T) {
	harness := newHTTPHarness(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	for _, event := range []Event{
		{Action: "unknown", Media: "movie"},
		{Action: "play", Media: "missing"},
		{Action: "play", Media: "photo"},
		{Action: "play", Media: "hidden"},
		{Action: "play", Media: "movie", Seconds: -1},
		{Action: "play", Media: "movie", Seconds: MaximumSeconds + 1},
		{Action: "play", Media: "movie", Seconds: math.NaN()},
		{Action: "play", Media: "movie", Seconds: math.Inf(1)},
	} {
		if harness.handler.validEvent(request, event) {
			t.Fatalf("event was accepted: %#v", event)
		}
	}
}

func serveRoomRequest(handler http.Handler, method, path, body, viewer, denied string) *httptest.ResponseRecorder { //nolint:unparam // Shared test requests intentionally use the leader fixture.
	request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Viewer", viewer)
	request.Header.Set("X-Deny", denied)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func dialHTTPRoom(t *testing.T, baseURL, room, viewer, denied string) *websocket.Conn {
	t.Helper()
	header := http.Header{"X-Viewer": {viewer}, "X-Deny": {denied}}
	connection, response, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(baseURL, "http")+"/api/v1/watch-rooms/"+room+"/events", &websocket.DialOptions{HTTPHeader: header})
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func readHTTPEvent(t *testing.T, connection *websocket.Conn) Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var event Event
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatal(err)
	}
	return event
}
