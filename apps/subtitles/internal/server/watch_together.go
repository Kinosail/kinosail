package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/watchrooms"
)

type watchRoomAdapter struct {
	rooms   *watchrooms.Rooms
	handler *watchrooms.HTTP
}

func newWatchRooms(ttl time.Duration, index *libraryIndex, auth *authentication) *watchRoomAdapter {
	rooms := watchrooms.New(ttl)
	config := watchrooms.HTTPConfig{Rooms: rooms, Index: index, Failure: localizedError, NotFound: localizedNotFound}
	config.ViewerID = func(request *http.Request) string { return currentViewer(request).ID }
	config.CanView = func(request *http.Request, item library.Item) bool { return canView(currentViewer(request), item) }
	config.Reauthorize = func(request *http.Request, route string) (*http.Request, string, bool) {
		return reauthorizeWatchRoom(auth, request, route)
	}
	return &watchRoomAdapter{rooms: rooms, handler: watchrooms.MustNewHTTP(config)}
}

func (rooms *watchRoomAdapter) register(mux *http.ServeMux, _ *libraryIndex, _ *authentication) {
	rooms.handler.Register(mux)
}

func (rooms *watchRoomAdapter) player(request *http.Request, _ *libraryIndex, item library.Item) (string, bool, []library.Item) {
	return rooms.handler.Player(request, item)
}

func (rooms *watchRoomAdapter) create(profile viewerProfile, media string, seconds float64) (string, bool) {
	return rooms.rooms.Create(profile.ID, media, seconds)
}

func roomMediaVisible(request *http.Request, index *libraryIndex, media string) bool {
	_, found := index.VisibleItem(request, media)
	return found
}

func reauthorizeWatchRoom(auth *authentication, request *http.Request, route string) (*http.Request, string, bool) {
	profile, found := auth.identity(request)
	if !found || !profile.Allowed(publicInternetRequest(request), time.Now()) || profile.APIKey && !profileAllowsAPI(profile, route) {
		return request, "", false
	}
	return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, profile)), profile.ID, true
}
