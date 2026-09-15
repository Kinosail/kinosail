package liveevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Access adapts app-owned identity and error presentation to the event stream.
type Access struct {
	Profile func(*http.Request) string
	Allowed func(*http.Request) bool
	Error   func(http.ResponseWriter, *http.Request, error, int)
}

// Register adds the versioned live event route.
func (hub *Hub) Register(mux *http.ServeMux, access Access) {
	mux.HandleFunc("GET /api/v1/events", hub.Handler(access))
}

// Handler serves the live event stream.
func (hub *Hub) Handler(access Access) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		subscription, controller, ok := hub.openStream(writer, request, access)
		if !ok {
			return
		}
		defer subscription.close()
		hub.stream(writer, request, access, subscription, controller)
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
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(writer, "retry: 3000\n\n")
	for _, event := range backlog {
		if writeEvent(writer, event) != nil {
			subscription.close()
			return nil, nil, false
		}
	}
	controller := http.NewResponseController(writer)
	if controller.Flush() != nil {
		subscription.close()
		return nil, nil, false
	}
	return subscription, controller, true
}

func (hub *Hub) stream(writer http.ResponseWriter, request *http.Request, access Access, subscription *subscription, controller *http.ResponseController) {
	heartbeat := time.NewTicker(hub.heartbeat)
	accessCheck := time.NewTicker(hub.accessCheck)
	defer heartbeat.Stop()
	defer accessCheck.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case event, open := <-subscription.events:
			if !open || !writeAndFlush(writer, controller, event) {
				return
			}
		case <-heartbeat.C:
			if !writeHeartbeat(writer, controller) {
				return
			}
		case <-accessCheck.C:
			if !access.Allowed(request) {
				return
			}
		}
	}
}

func writeAndFlush(writer http.ResponseWriter, controller *http.ResponseController, event Event) bool {
	return writeEvent(writer, event) == nil && controller.Flush() == nil
}

func writeHeartbeat(writer http.ResponseWriter, controller *http.ResponseController) bool {
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
