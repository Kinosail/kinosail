package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
)

func (api *jellyfinAPI) registerItems(mux *http.ServeMux) {
	sharedjellyfin.RegisterItems(mux, sharedjellyfin.ItemHandlers{
		Views: ownJellyfinUser(api.views), Items: ownJellyfinUser(api.items), Latest: ownJellyfinUser(api.latest), Persons: api.persons, Item: ownJellyfinUser(api.item),
		Seasons: api.seasons, Episodes: api.episodes, NextUp: api.nextUp, Resume: api.resume,
	})
}

func ownJellyfinUser(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if user := request.PathValue("user"); user != "" && user != currentViewer(request).ID {
			http.NotFound(writer, request)
			return
		}
		next(writer, request)
	}
}

func (api *jellyfinAPI) views(writer http.ResponseWriter, request *http.Request) {
	view, err := visibleView(request, api.index)
	if err != nil {
		library.MarkLoadingRetry(writer.Header(), err)
		http.Error(writer, "library is unavailable", http.StatusServiceUnavailable)
		return
	}
	jellyfinJSON(writer, api.listing(request, view).Views())
}

func (api *jellyfinAPI) items(writer http.ResponseWriter, request *http.Request) {
	view, err := visibleView(request, api.index)
	if err != nil {
		library.MarkLoadingRetry(writer.Header(), err)
		http.Error(writer, "library is unavailable", http.StatusServiceUnavailable)
		return
	}
	result, err := api.listing(request, view).Items(request.URL.Query())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) item(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result, found := sharedjellyfin.NewCatalog(view, func(item library.Item) sharedjellyfin.ItemRecord { //nolint:contextcheck // The projection uses the captured request context to inspect the item.
		return api.itemDTO(request, item, api.userDataDTO, jellyfinCatalogSource(item, api.probe.inspect(request.Context(), item)))
	}).Item(request.PathValue("id"))
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) seasons(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result, found := api.listing(request, view).SeasonsResult(request.PathValue("id"))
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) episodes(writer http.ResponseWriter, request *http.Request) {
	view, _ := visibleView(request, api.index)
	result, found := api.listing(request, view).Episodes(request.PathValue("id"), request.URL.Query())
	if !found {
		http.NotFound(writer, request)
		return
	}
	jellyfinJSON(writer, result)
}

func (api *jellyfinAPI) listing(request *http.Request, view *library.View) sharedjellyfin.Catalog {
	sources := sharedjellyfin.HasField(request.URL.Query(), "MediaSources")
	return sharedjellyfin.NewCatalog(view, func(item library.Item) sharedjellyfin.ItemRecord {
		var source map[string]any
		if sources {
			media, _ := api.probe.cachedFacts(item)
			source = jellyfinCatalogSource(item, media)
		}
		return api.itemDTO(request, item, api.listingUserDataDTO, source)
	})
}

func (api *jellyfinAPI) itemDTO(request *http.Request, item library.Item, userData func(*http.Request, library.Item) sharedjellyfin.UserData, source map[string]any) sharedjellyfin.ItemRecord {
	viewer := currentViewer(request)
	return sharedjellyfin.ItemDTO(item, sharedjellyfin.ItemOptions{
		CanDownload: viewer.Owner || viewer.Downloads,
		UserData:    userData(request, item),
		MediaSource: source,
	})
}
