package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
)

func (api *jellyfinAPI) persons(writer http.ResponseWriter, request *http.Request) {
	sharedjellyfin.Listing(writer, request, sharedjellyfin.Result(make([]map[string]any, 0), 0))
}

func (api *jellyfinAPI) latest(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result, err := api.listing(request, view).Latest(request.URL.Query())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	sharedjellyfin.Listing(writer, request, result)
}

func (api *jellyfinAPI) resume(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result := api.listing(request, view).Resume(func(item library.Item) sharedjellyfin.UserState {
		state := api.progress.Get(request, item.ID)
		return sharedjellyfin.UserState{Seconds: state.Seconds, Watched: state.Watched}
	})
	sharedjellyfin.Listing(writer, request, result)
}

func (api *jellyfinAPI) nextUp(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result := api.listing(request, view).NextUp(func(item library.Item) bool { return api.progress.Watched(request, item.ID) })
	sharedjellyfin.Listing(writer, request, result)
}

func jellyfinPlaySessionQuery(request *http.Request) string {
	return sharedjellyfin.Query(request.URL.Query(), "playSessionId")
}
