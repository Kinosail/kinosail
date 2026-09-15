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
	handler := watchrooms.MustNewHTTP(watchrooms.HTTPConfig{
		Rooms: rooms, Index: index,
		ViewerID: func(request *http.Request) string { return currentViewer(request).ID },
		CanView:  func(request *http.Request, item library.Item) bool { return canView(currentViewer(request), item) },
		Reauthorize: func(request *http.Request, route string) (*http.Request, string, bool) {
			return reauthorizeWatchRoom(auth, request, route)
		},
		Failure: localizedError, NotFound: localizedNotFound,
	})
	return &watchRoomAdapter{rooms: rooms, handler: handler}
}

func (rooms *watchRoomAdapter) create(profile viewerProfile, media string, seconds float64) (string, bool) {
	return rooms.rooms.Create(profile.ID, media, seconds)
}

func (rooms *watchRoomAdapter) register(mux *http.ServeMux, _ *libraryIndex, _ *authentication) {
	rooms.handler.Register(mux)
}

func (rooms *watchRoomAdapter) player(request *http.Request, _ *libraryIndex, item library.Item) (string, bool, []library.Item) {
	return rooms.handler.Player(request, item)
}

func reauthorizeWatchRoom(auth *authentication, request *http.Request, route string) (*http.Request, string, bool) {
	profile, found := auth.identity(request)
	if !found || !profile.Allowed(publicInternetRequest(request), time.Now()) || profile.APIKey && !profileAllowsAPI(profile, route) {
		return request, "", false
	}
	ctx := context.WithValue(request.Context(), viewerContextKey{}, profile)
	return request.WithContext(ctx), profile.ID, true
}

func roomMediaVisible(request *http.Request, index *libraryIndex, media string) bool {
	_, found := index.VisibleItem(request, media)
	return found
}
