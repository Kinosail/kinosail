package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
)

func (api *jellyfinAPI) nextUp(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result := api.catalog(request, items).NextUp(func(item library.Item) bool { return api.progress.Watched(request, item.ID) })
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) resume(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	state := func(item library.Item) sharedjellyfin.UserState {
		progress := api.progress.Get(request, item.ID)
		return sharedjellyfin.UserState{Seconds: progress.Seconds, Watched: progress.Watched}
	}
	jellyfinJSON(writer, api.catalog(request, items).Resume(state))
}

func (api *jellyfinAPI) latest(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result, err := api.catalog(request, items).Latest(request.URL.Query())
	if err == nil {
		jellyfinJSON(writer, result)
		return
	}
	http.Error(writer, err.Error(), http.StatusBadRequest)
}

func (api *jellyfinAPI) persons(writer http.ResponseWriter, _ *http.Request) {
	jellyfinJSON(writer, sharedjellyfin.Result([]map[string]any{}, 0))
}

func jellyfinPlaySessionID(request *http.Request) string {
	return sharedjellyfin.Query(request.URL.Query(), "playSessionId")
}
