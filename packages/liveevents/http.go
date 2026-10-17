package liveevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Access struct {
	Profile func(*http.Request) string
	Allowed func(*http.Request) bool
	Changed func() <-chan struct{}
	Error   func(http.ResponseWriter, *http.Request, error, int)
}

func (access Access) changes() <-chan struct{} {
	if access.Changed == nil {
		return nil
	}
	return access.Changed()
}

func (hub *Hub) Register(mux *http.ServeMux, access Access) {
	mux.HandleFunc("GET /api/v1/events", hub.Handler(access))
}

func (hub *Hub) Handler(access Access) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		changed := access.changes()
		subscription, controller, ok := hub.openStream(writer, request, access)
		if !ok {
			return
		}
		defer subscription.close()
		hub.stream(writer, request, access, subscription, controller, changed)
	}
}

func (hub *Hub) openStream(writer http.ResponseWriter, request *http.Request, access Access) (*subscription, *http.ResponseController, bool) {
	after, err := lastEventID(request.Header.Get("Last-Event-ID"))
	if err != nil {
		access.Error(writer, request, err, http.StatusBadRequest)
		return nil, nil, false
	}
	backlog, subscription, ok := hub.subscribe(access.Profile(request), after)
	if !ok {
		access.Error(writer, request, errors.New("too many event streams"), http.StatusTooManyRequests)
		return nil, nil, false
	}
	if !access.Allowed(request) {
		subscription.close()
		access.Error(writer, request, errors.New("event stream access was revoked"), http.StatusUnauthorized)
		return nil, nil, false
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(writer)
	hub.extendWrite(controller)
	_, _ = fmt.Fprint(writer, "retry: 3000\n\n")
	for _, event := range backlog {
		if writeEvent(writer, event) != nil {
			subscription.close()
			return nil, nil, false
		}
	}
	if controller.Flush() != nil {
		subscription.close()
		return nil, nil, false
	}
	return subscription, controller, true
}

func (hub *Hub) stream(writer http.ResponseWriter, request *http.Request, access Access, subscription *subscription, controller *http.ResponseController, changed <-chan struct{}) {
	heartbeat := time.NewTicker(hub.heartbeat)
	accessCheck := time.NewTicker(hub.accessCheck)
	defer heartbeat.Stop()
	defer accessCheck.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case event, open := <-subscription.events:
			if !open || !hub.writeAndFlush(writer, controller, event) {
				return
			}
		case <-heartbeat.C:
			if !hub.writeHeartbeat(writer, controller) {
				return
			}
		case <-changed:
			changed = access.changes()
			if !access.Allowed(request) {
				return
			}
		case <-accessCheck.C:
			if !access.Allowed(request) {
				return
			}
		}
	}
}

func (hub *Hub) extendWrite(controller *http.ResponseController) {
	_ = controller.SetWriteDeadline(time.Now().Add(hub.writeTimeout))
}

func (hub *Hub) writeAndFlush(writer http.ResponseWriter, controller *http.ResponseController, event Event) bool {
	hub.extendWrite(controller)
	return writeEvent(writer, event) == nil && controller.Flush() == nil
}

func (hub *Hub) writeHeartbeat(writer http.ResponseWriter, controller *http.ResponseController) bool {
	hub.extendWrite(controller)
	_, _ = fmt.Fprint(writer, ": keepalive\n\n")
	return controller.Flush() == nil
}

func lastEventID(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	if len(value) > 20 {
		return 0, errors.New("Last-Event-ID is invalid")
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("Last-Event-ID is invalid")
	}
	return id, nil
}

func writeEvent(writer http.ResponseWriter, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data)
	return err
}
