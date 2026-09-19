package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playerweb"
)

const playlistHTML = playerweb.PlaylistHTML

var playlistView = newLocalizedTemplate("playlist", playlistHTMLWithExport)

func playlistHandlers(index *libraryIndex, store *listStore) catalog.PlaylistHandlers {
	return catalog.NewPlaylistHandlers(catalog.PlaylistHandlersConfig{
		VisibleLibrary: func(request *http.Request) ([]library.Item, error) {
			return visibleLibrary(request, index)
		},
		PlaylistNames: store.PlaylistNames,
		Playlist:      store.Playlist,
		PlaylistRule:  store.playlistRule,
		VisibleItem: func(request *http.Request, id string) (library.Item, bool) {
			return visibleItem(request, index, id)
		},
		Viewer: func(request *http.Request) string {
			return currentViewer(request).ID
		},
		SetPlaylist: store.SetPlaylist,
		Order:       store.Order,
		Render: func(writer http.ResponseWriter, request *http.Request, page catalog.PlaylistPage) error {
			return playlistView.Execute(writer, request, page)
		},
		Failure:  localizedError,
		NotFound: localizedNotFound,
	})
}

func browsePlaylist(index *libraryIndex, store *listStore) http.HandlerFunc {
	return playlistHandlers(index, store).Browse
}

func orderPlaylist(index *libraryIndex, store *listStore) http.HandlerFunc {
	return playlistHandlers(index, store).Order
}

func managePlaylistItem(index *libraryIndex, store *listStore) http.HandlerFunc {
	return playlistHandlers(index, store).ManageItem
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
