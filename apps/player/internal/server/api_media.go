package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func registerMediaAPI(mux *http.ServeMux, index *libraryIndex, progress *progressStore, lists *listStore, auth *authentication) {
	handlers := catalogapi.NewMediaHandlers(index, progress, lists, func(request *http.Request) string {
		return currentViewer(request).ID
	}, timelineFromPlaybackToken, apiStoreStatus)
	handlers.Register(mux, catalogapi.MediaExtras{PlaybackEvents: apiPlaybackTrace(index), Reader: apiReader(index), ReaderProgress: apiReaderProgress(index, progress), Owner: auth.owner})
}
