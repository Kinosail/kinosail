package watchrooms

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	roomEventsRoute  = "GET /api/v1/watch-rooms/{id}/events"
	accessBackstop   = 15 * time.Second
	roomWriteTimeout = 10 * time.Second
)

type HTTPIndex interface {
	Find(string) (library.Item, bool)
	Safe(string) bool
	Snapshot() ([]library.Item, error)
}

type HTTPConfig struct {
	Rooms       *Rooms
	Index       HTTPIndex
	ViewerID    func(*http.Request) string
	CanView     func(*http.Request, library.Item) bool
	Reauthorize func(*http.Request, string) (*http.Request, string, bool)
	Changed     func() <-chan struct{}
	Failure     func(http.ResponseWriter, *http.Request, string, int)
	NotFound    func(http.ResponseWriter, *http.Request)
}

type HTTP struct {
	config HTTPConfig
}

func NewHTTP(config HTTPConfig) (*HTTP, error) {
	if config.Rooms == nil || config.Index == nil || config.ViewerID == nil || config.CanView == nil || config.Reauthorize == nil || config.Failure == nil || config.NotFound == nil {
		return nil, errors.New("watch room HTTP configuration is invalid")
	}
	return &HTTP{config: config}, nil
}

func MustNewHTTP(config HTTPConfig) *HTTP {
	handler, err := NewHTTP(config)
	if err != nil {
		panic(err)
	}
	return handler
}

func (handler *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /watch-together", handler.create)
	mux.HandleFunc("GET /room/{id}", handler.redirect)
	mux.HandleFunc("GET /api/v1/watch-rooms/{id}/events", handler.connect)
	mux.HandleFunc("GET /watch-together/{id}", handler.connect)
}

func (handler *HTTP) Player(request *http.Request, item library.Item) (string, bool, []library.Item) {
	id := request.URL.Query().Get("room")
	event, found := handler.config.Rooms.Snapshot(id, handler.config.ViewerID(request))
	if !found || event.Media != item.ID || !handler.mediaVisible(request, event.Media) {
		return "", false, nil
	}
	items, _ := handler.config.Index.Snapshot()
	playable := make([]library.Item, 0, len(items))
	for _, candidate := range items {
		if handler.config.CanView(request, candidate) && (candidate.Kind == "video" || candidate.Kind == "audio") {
			playable = append(playable, candidate)
		}
	}
	return id, event.Leader, playable
}

func (handler *HTTP) create(writer http.ResponseWriter, request *http.Request) {
	item, found := handler.visibleItem(request, request.FormValue("media"))
	seconds, err := strconv.ParseFloat(request.FormValue("seconds"), 64)
	if request.FormValue("seconds") == "" {
		seconds, err = 0, nil
	}
	if !found || item.Kind == "photo" || err != nil || !validHTTPSeconds(seconds) {
		handler.config.Failure(writer, request, "Watch Together state is invalid", http.StatusBadRequest)
		return
	}
	id, created := handler.config.Rooms.Create(handler.config.ViewerID(request), item.ID, seconds)
	if !created {
		handler.config.Failure(writer, request, "too many Watch Together rooms", http.StatusTooManyRequests)
		return
	}
	http.Redirect(writer, request, "/watch/"+item.ID+"?room="+url.QueryEscape(id), http.StatusSeeOther)
}

func (handler *HTTP) redirect(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	event, found := handler.config.Rooms.Snapshot(id, handler.config.ViewerID(request))
	if !found || !handler.mediaVisible(request, event.Media) {
		handler.config.NotFound(writer, request)
		return
	}
	http.Redirect(writer, request, "/watch/"+event.Media+"?room="+url.QueryEscape(id), http.StatusSeeOther)
}

func (handler *HTTP) connect(writer http.ResponseWriter, request *http.Request) {
	joined, ok := handler.join(writer, request)
	if !ok {
		return
	}
	defer joined.connection.Close(websocket.StatusNormalClosure, "room left")
	defer joined.subscription.Close()
	var current atomic.Pointer[http.Request]
	current.Store(request)
	go handler.writeEvents(request, &current, joined.connection, joined.subscription)
	stop := make(chan struct{})
	go handler.monitorAccess(stop, joined.connection, request, &current, joined.id, joined.subscription)
	defer close(stop)
	for {
		joined.event = Event{}
		if wsjson.Read(request.Context(), joined.connection, &joined.event) != nil {
			return
		}
		if joined.event.Action == "observe" {
			if !handler.config.Rooms.ObserveDrift(joined.subscription, joined.event.Drift) {
				return
			}
			continue
		}
		if !handler.config.Rooms.Update(joined.subscription, joined.event, handler.validEvent(joined.request, joined.event)) {
			return
		}
	}
}

type joinedHTTPRoom struct {
	connection   *websocket.Conn
	subscription *Subscription
	request      *http.Request
	event        Event
	id           string
}

func (handler *HTTP) join(writer http.ResponseWriter, request *http.Request) (joinedHTTPRoom, bool) {
	joined := joinedHTTPRoom{id: request.PathValue("id"), request: request}
	viewer := handler.config.ViewerID(request)
	var found bool
	joined.event, found = handler.config.Rooms.Snapshot(joined.id, viewer)
	if !found || !handler.mediaVisible(request, joined.event.Media) {
		handler.config.NotFound(writer, request)
		return joinedHTTPRoom{}, false
	}
	var err error
	joined.connection, err = websocket.Accept(writer, request, nil)
	if err != nil {
		return joinedHTTPRoom{}, false
	}
	joined.connection.SetReadLimit(4096)
	joined.event, joined.subscription, found = handler.config.Rooms.Join(joined.id, viewer)
	if !found || !handler.mediaVisible(request, joined.event.Media) || writeRoomEvent(request, joined.connection, joined.event) != nil {
		if joined.subscription != nil {
			joined.subscription.Close()
		}
		_ = joined.connection.CloseNow()
		return joinedHTTPRoom{}, false
	}
	return joined, true
}

func (handler *HTTP) writeEvents(request *http.Request, current *atomic.Pointer[http.Request], connection *websocket.Conn, subscription *Subscription) {
	viewer := handler.config.ViewerID(request)
	for event := range subscription.Events() {
		media := event.Media
		if event.Action == "denied" {
			room, _ := handler.config.Rooms.Snapshot(subscription.roomID, viewer)
			media = room.Media
		}
		if !handler.mediaAllowed(current.Load(), media) || writeRoomEvent(request, connection, event) != nil {
			subscription.Close()
			break
		}
	}
	_ = connection.CloseNow()
}

func writeRoomEvent(request *http.Request, connection *websocket.Conn, event Event) error {
	ctx, cancel := context.WithTimeout(request.Context(), roomWriteTimeout)
	defer cancel()
	return wsjson.Write(ctx, connection, event)
}

func (handler *HTTP) monitorAccess(stop <-chan struct{}, connection *websocket.Conn, request *http.Request, current *atomic.Pointer[http.Request], roomID string, subscription *Subscription) {
	ticker := time.NewTicker(accessBackstop)
	defer ticker.Stop()
	changed := handler.changes()
	for handler.stillAllowed(request, current, roomID) {
		select {
		case <-stop:
			return
		case <-changed:
			changed = handler.changes()
		case <-ticker.C:
		}
	}
	subscription.Close()
	_ = connection.CloseNow()
}

func (handler *HTTP) changes() <-chan struct{} {
	if handler.config.Changed == nil {
		return nil
	}
	return handler.config.Changed()
}

func (handler *HTTP) stillAllowed(request *http.Request, current *atomic.Pointer[http.Request], roomID string) bool {
	fresh, viewer, found := handler.config.Reauthorize(request, roomEventsRoute)
	if !found {
		return false
	}
	current.Store(fresh)
	event, active := handler.config.Rooms.Snapshot(roomID, viewer)
	return active && handler.mediaAllowed(fresh, event.Media)
}

func (handler *HTTP) mediaAllowed(request *http.Request, media string) bool {
	item, found := handler.config.Index.Find(media)
	return found && handler.config.CanView(request, item)
}

func (handler *HTTP) visibleItem(request *http.Request, id string) (library.Item, bool) {
	item, found := handler.config.Index.Find(id)
	return item, found && handler.config.Index.Safe(item.Path) && handler.config.CanView(request, item)
}

func (handler *HTTP) mediaVisible(request *http.Request, media string) bool {
	_, found := handler.visibleItem(request, media)
	return found
}

func (handler *HTTP) validEvent(request *http.Request, event Event) bool {
	item, found := handler.config.Index.Find(event.Media)
	playable := item.Kind == "video" || item.Kind == "audio"
	return slicesContains([]string{"play", "pause", "seek", "media"}, event.Action) && found && playable && handler.config.CanView(request, item) && validHTTPSeconds(event.Seconds)
}

func validHTTPSeconds(seconds float64) bool {
	return seconds >= 0 && seconds <= MaximumSeconds && !math.IsNaN(seconds) && !math.IsInf(seconds, 0)
}

func slicesContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
