package catalogapi

import (
	"errors"
	"net/http"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

func (handlers MediaHandlers) audioQueue(writer http.ResponseWriter, request *http.Request) {
	items := handlers.Index.VisibleLibrary(request)
	queue, found := AudioQueue(items, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	apiAction{body: map[string]any{"items": handlers.Progress.ClientItems(request, queue)}, status: http.StatusOK}.serve(writer)
}

// AudioQueue returns the selected track and every later track in its album.
func AudioQueue(items []library.Item, id string) ([]library.Item, bool) {
	_, albums := library.OrganizeMusic(items)
	for _, album := range albums {
		for position, track := range album.Tracks {
			if track.ID == id {
				return album.Tracks[position:], true
			}
		}
	}
	return nil, false
}

func (handlers MediaHandlers) history(writer http.ResponseWriter, request *http.Request) {
	items := handlers.Index.VisibleLibrary(request)
	items = handlers.Progress.History(request, items)
	apiAction{body: map[string]any{"items": handlers.Progress.ClientItems(request, items)}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) item(writer http.ResponseWriter, request *http.Request) {
	item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
	if !found {
		apiAction{notFound: true}.serve(writer)
		return
	}
	apiAction{body: map[string]any{"item": handlers.Progress.ClientItem(request, item), "listed": handlers.Lists.Has(request, item.ID), "profileId": handlers.Viewer(request)}, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) playlists(writer http.ResponseWriter, request *http.Request) {
	items := handlers.Index.VisibleLibrary(request)
	body := map[string]any{"playlists": handlers.Lists.PlaylistNames(request), "summaries": handlers.Lists.PlaylistSummariesJSON(request, items, "")}
	apiAction{body: body, status: http.StatusOK}.serve(writer)
}

func (handlers MediaHandlers) playlist(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if len(query) > 0 {
		values, ok := query["format"]
		if !ok || len(values) != 1 || values[0] != PlaylistExportQuery {
			apiAction{err: errors.New("playlist format is invalid"), status: http.StatusBadRequest}.serve(writer)
			return
		}
		document, err := handlers.Lists.ExportPlaylistDocument(handlers.Viewer(request), request.PathValue("name"))
		if err != nil {
			apiAction{notFound: true}.serve(writer)
			return
		}
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-playlist.json"`)
		apiAction{body: document, status: http.StatusOK}.serve(writer)
		return
	}
	name := request.PathValue("name")
	if !slices.Contains(handlers.Lists.PlaylistNames(request), name) {
		apiAction{notFound: true}.serve(writer)
		return
	}
	items := handlers.Index.VisibleLibrary(request)
	body := map[string]any{"name": name, "items": handlers.Progress.ClientItems(request, handlers.Lists.Playlist(request, name, items))}
	apiAction{body: body, status: http.StatusOK}.serve(writer)
}
