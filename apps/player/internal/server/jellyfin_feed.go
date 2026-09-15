package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
)

func (api *jellyfinAPI) persons(writer http.ResponseWriter, _ *http.Request) {
	jellyfinJSON(writer, sharedjellyfin.Result(make([]map[string]any, 0), 0))
}

func (api *jellyfinAPI) latest(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result, err := api.catalog(request, items).Latest(request.URL.Query())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) resume(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result := api.catalog(request, items).Resume(func(item library.Item) sharedjellyfin.UserState {
		state := api.progress.Get(request, item.ID)
		return sharedjellyfin.UserState{Seconds: state.Seconds, Watched: state.Watched}
	})
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) nextUp(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result := api.catalog(request, items).NextUp(func(item library.Item) bool { return api.progress.Watched(request, item.ID) })
	jellyfinJSON(writer, result)
}

func jellyfinPlaySessionQuery(request *http.Request) string {
	return sharedjellyfin.Query(request.URL.Query(), "playSessionId")
}
