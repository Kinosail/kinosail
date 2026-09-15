package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type syncEvent struct {
	Action      string    `json:"action"`
	Media       string    `json:"media"`
	Seconds     float64   `json:"seconds"`
	Leader      bool      `json:"leader"`
	Revision    uint64    `json:"revision,omitempty"`
	EffectiveAt time.Time `json:"effectiveAt,omitempty"`
	Drift       float64   `json:"drift,omitempty"`
}

//nolint:cyclop,funlen // One workflow test covers room state transitions and expiry.
func TestWatchTogetherEnforcesLeaderSyncAndExpiry(t *testing.T) { //nolint:cyclop,funlen // One workflow test covers room state transitions and expiry.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	for _, name := range []string{"Movie.mp4", "Next.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true, WatchRoomTTL: time.Second})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	addTestViewer(t, handler, owner)
	viewer := signInTestProfile(t, handler, "/login", "name=Sam&password=viewer-password")
	home := requestWithCookie(t, handler, http.MethodGet, "/", "", owner)
	ids := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindAllStringSubmatch(home.Body.String(), -1)
	invalid := requestWithCookie(t, handler, http.MethodPost, "/watch-together", "media="+url.QueryEscape(ids[0][1])+"&seconds=999999999", owner)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("oversized room position = %d %q", invalid.Code, invalid.Body.String())
	}
	room := createTestRoom(t, handler, owner, ids[0][1])
	page := requestWithCookie(t, handler, http.MethodGet, room, "", owner)
	mustRoomPage(t, page)
	web := httptest.NewServer(handler)
	t.Cleanup(web.Close)
	roomID := regexp.MustCompile(`room=([^&]+)`).FindStringSubmatch(room)[1]
	leader := dialRoom(t, web.URL, roomID, owner)
	participant := dialRoom(t, web.URL, roomID, viewer)
	initial := readEvent(t, leader)
	readEvent(t, participant)
	writeEvent(t, participant, syncEvent{Action: "play", Media: ids[0][1], Seconds: 8})
	if denied := readEvent(t, participant); denied.Action != "denied" {
		t.Fatalf("participant event = %+v", denied)
	}
	writeEvent(t, leader, syncEvent{Action: "play", Media: ids[0][1], Seconds: 42})
	if synced := readEvent(t, participant); synced.Action != "play" || synced.Seconds != 42 || synced.Leader || synced.Revision <= initial.Revision || synced.EffectiveAt.IsZero() {
		t.Fatalf("synced event = %+v", synced)
	}
	writeEvent(t, leader, syncEvent{Action: "media", Media: ids[1][1], Seconds: 0})
	if changed := readEvent(t, participant); changed.Media != ids[1][1] {
		t.Fatalf("media event = %+v", changed)
	}
	_ = participant.Close(websocket.StatusNormalClosure, "done")
	participant = dialRoom(t, web.URL, roomID, viewer)
	if resumed := readEvent(t, participant); resumed.Media != ids[1][1] || resumed.Action != "media" {
		t.Fatalf("reconnected event = %+v", resumed)
	}
	requestWithCookie(t, handler, http.MethodPost, "/settings/profiles/remove", "id="+storedProfileID(t, dataDir, "Sam"), owner)
	closed, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := wsjson.Read(closed, participant, &syncEvent{}); err == nil {
		t.Fatal("removed Viewer retained an active Watch Together connection")
	}
	_ = leader.Close(websocket.StatusNormalClosure, "done")
	_ = participant.Close(websocket.StatusNormalClosure, "done")
	time.Sleep(1100 * time.Millisecond)
	expired := requestWithCookie(t, handler, http.MethodGet, "/room/"+roomID, "", owner)
	if expired.Code != http.StatusNotFound {
		t.Fatalf("expired room = %d %q", expired.Code, expired.Body.String())
	}
}

func TestWatchTogetherPlayerConnectsAndReconnectsThroughTheRoomAPI(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))
	for _, expected := range []string{`new WebSocket`, `/api/v1/watch-rooms/`, `player.dataset.roomLeader`, `event.effectiveAt`, `sendRoomEvent("play")`, `sendRoomEvent("media"`, `setTimeout(connectRoom`, `socket?.close(1000`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("Watch Together browser client lacks %q", expected)
		}
	}
}

func TestWatchTogetherRejectsInvalidDriftWithoutMetricSideEffect(t *testing.T) {
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	home := requestWithCookie(t, handler, http.MethodGet, "/", "", owner)
	media := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	room := createTestRoom(t, handler, owner, media)
	web := httptest.NewServer(handler)
	t.Cleanup(web.Close)
	connection := dialRoom(t, web.URL, regexp.MustCompile(`room=([^&]+)`).FindStringSubmatch(room)[1], owner)
	readEvent(t, connection)
	writeEvent(t, connection, syncEvent{Action: "observe", Drift: -1})
	closed, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := wsjson.Read(closed, connection, &syncEvent{}); err == nil {
		t.Fatal("invalid drift retained the Watch Together connection")
	}
	metrics := requestWithCookie(t, handler, http.MethodGet, "/settings/metrics", "", owner)
	if !strings.Contains(metrics.Body.String(), "kinosail_watch_room_drift_observations_total 0\n") {
		t.Fatalf("invalid drift changed metrics: %q", metrics.Body.String())
	}
}

func addTestViewer(t *testing.T, handler http.Handler, owner *http.Cookie) {
	t.Helper()
	response := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", "name=Sam&password=viewer-password&rating=all&libraries=all", owner)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("add Viewer = %d %q", response.Code, response.Body.String())
	}
}

func mustRoomPage(t *testing.T, page *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(page.Body.String(), `data-room-leader="true"`) || !strings.Contains(page.Body.String(), `role="status" aria-live="polite" data-room-state`) {
		t.Fatalf("leader page = %d %q", page.Code, page.Body.String())
	}
}

func createTestRoom(t *testing.T, handler http.Handler, owner *http.Cookie, media string) string {
	t.Helper()
	response := requestWithCookie(t, handler, http.MethodPost, "/watch-together", "media="+url.QueryEscape(media), owner)
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "room=") {
		t.Fatalf("create room = %d %q", response.Code, response.Header().Get("Location"))
	}
	return response.Header().Get("Location")
}

func requestWithCookie(t *testing.T, handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func dialRoom(t *testing.T, base, room string, cookie *http.Cookie) *websocket.Conn {
	t.Helper()
	connection, response, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(base, "http")+"/api/v1/watch-rooms/"+room+"/events", &websocket.DialOptions{HTTPHeader: http.Header{"Cookie": {cookie.String()}}})
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func readEvent(t *testing.T, connection *websocket.Conn) syncEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var event syncEvent
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func writeEvent(t *testing.T, connection *websocket.Conn, event syncEvent) {
	t.Helper()
	if err := wsjson.Write(t.Context(), connection, event); err != nil {
		t.Fatal(err)
	}
}
