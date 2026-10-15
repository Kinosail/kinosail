package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func registerFiles(mux *http.ServeMux, index *libraryIndex) {
	mux.HandleFunc("GET /media/{id}", serveFile(index, playback.MediaPath, ""))
	mux.HandleFunc("GET /art/{id}", serveFile(index, playback.ArtworkPath, ""))
}

func serveFile(index *libraryIndex, selectPath func(library.Item) string, contentType string) http.HandlerFunc {
	return playback.LibraryFileHandler(playback.LibraryFileDependencies{Lookup: index.VisibleItem, Select: selectPath, Safe: index.Safe, NotFound: localizedNotFound, ContentType: contentType})
}
