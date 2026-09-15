package productapi

import (
	"errors"
	"math"
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

// RoomService is the app-owned visibility and watch-room port.
type RoomService interface {
	Visible(*http.Request, string) (library.Item, bool)
	Viewer(*http.Request) identitycore.Profile
	Create(identitycore.Profile, string, float64) (string, bool)
}

type roomInput struct {
	Media   string
	Seconds float64
}

// CreateRoom validates and creates one Player watch room.
func CreateRoom(service RoomService) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" || !jsonRequest(request) {
			apihttp.Error(writer, errors.New("watch room state is invalid"), http.StatusBadRequest)
			return
		}
		var input roomInput
		if err := decodeObjectRequest(writer, request, &input); err != nil {
			apihttp.Error(writer, err, http.StatusBadRequest)
			return
		}
		if !validRoomInput(input) {
			apihttp.Error(writer, errors.New("watch room state is invalid"), http.StatusBadRequest)
			return
		}
		item, found := service.Visible(request, input.Media)
		if !found || item.Kind == "photo" {
			apihttp.Error(writer, errors.New("watch room state is invalid"), http.StatusBadRequest)
			return
		}
		id, created := service.Create(service.Viewer(request), item.ID, input.Seconds)
		if !created {
			apihttp.Error(writer, errors.New("too many watch rooms"), http.StatusTooManyRequests)
			return
		}
		apihttp.WriteJSON(writer, map[string]string{"id": id, "watch": "/watch/" + item.ID + "?room=" + id, "socket": "/api/v1/watch-rooms/" + id + "/events"}, http.StatusCreated)
	}
}

func validRoomInput(input roomInput) bool {
	return validItemID(input.Media) && input.Seconds >= 0 && !math.IsNaN(input.Seconds) && !math.IsInf(input.Seconds, 0)
}
