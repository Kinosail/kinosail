package server

import (
	"net/http"
	"time"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
)

var jellyfinLibraryRoutes = routeSet(sharedjellyfin.LibraryRoutes()...)

func (api *jellyfinAPI) users(writer http.ResponseWriter, _ *http.Request) {
	sharedjellyfin.Users(writer, api.auth.profiles.list(), func(profile viewerProfile) (map[string]any, bool) {
		return api.userDTO(profile), !profile.SCIMDeleted
	}, jellyfinJSON)
}

func (api *jellyfinAPI) createAuthKey(writer http.ResponseWriter, request *http.Request) {
	profile := currentViewer(request)
	sharedjellyfin.CreateAuthKey(writer, request, profile.ID, func(app string) (string, error) {
		return api.auth.profiles.createJellyfinAPIKey(profile, app)
	}, sessionKey, &api.keyGrants, time.Now)
}

func (api *jellyfinAPI) authKeys(writer http.ResponseWriter, request *http.Request) {
	sharedjellyfin.AuthKeys(writer, currentViewer(request).ID, &api.keyGrants, time.Now(), jellyfinJSON)
}
