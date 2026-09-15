package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
)

func (api *jellyfinAPI) registerItems(mux *http.ServeMux) {
	sharedjellyfin.RegisterItems(mux, sharedjellyfin.ItemHandlers{
		Views: api.views, Items: api.items, Latest: api.latest, Persons: api.persons, Item: api.item,
		Seasons: api.seasons, Episodes: api.episodes, NextUp: api.nextUp, Resume: api.resume,
	})
}

func (api *jellyfinAPI) views(writer http.ResponseWriter, request *http.Request) {
	items, err := visibleLibrary(request, api.index)
	if err != nil {
		http.Error(writer, "library is unavailable", http.StatusServiceUnavailable)
		return
	}
	jellyfinJSON(writer, api.catalog(request, items).Views())
}

func (api *jellyfinAPI) items(writer http.ResponseWriter, request *http.Request) {
	items, err := visibleLibrary(request, api.index)
	if err != nil {
		http.Error(writer, "library is unavailable", http.StatusServiceUnavailable)
		return
	}
	result, err := api.catalog(request, items).Items(request.URL.Query())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) item(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result, found := api.catalog(request, items).Item(request.PathValue("id"))
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) seasons(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result, found := api.catalog(request, items).SeasonsResult(request.PathValue("id"))
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) episodes(writer http.ResponseWriter, request *http.Request) {
	items, _ := visibleLibrary(request, api.index)
	result, found := api.catalog(request, items).Episodes(request.PathValue("id"), request.URL.Query())
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) catalog(request *http.Request, items []library.Item) sharedjellyfin.Catalog {
	return sharedjellyfin.NewCatalog(items, func(item library.Item) map[string]any { return api.itemDTO(request, item) })
}

func (api *jellyfinAPI) itemDTO(request *http.Request, item library.Item) map[string]any {
	viewer := currentViewer(request)
	return sharedjellyfin.ItemDTO(item, sharedjellyfin.ItemOptions{
		CanDownload: viewer.Owner || viewer.Downloads,
		UserData:    api.userDataDTO(request, item),
		MediaSource: jellyfinMediaSource(item, mediaFactsFor(item, probeResult{}), PlaybackPlan{Allowed: true, Mode: "direct", Reason: "catalog-summary", SubtitleMode: "none", ColorMode: "preserve"}, "", ""),
	})
}
