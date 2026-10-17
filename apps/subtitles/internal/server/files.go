package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func registerFiles(mux *http.ServeMux, index *libraryIndex, auth *authentication) {
	mux.Handle("GET /media/{id}", auth.owner(serveFile(index, playback.MediaPath, "")))
	mux.HandleFunc("GET /art/{id}", playback.LibraryFileHandler(playback.LibraryFileDependencies{Lookup: index.VisibleItem, Select: playback.ArtworkPath, Safe: index.Safe, NotFound: localizedNotFound, Revalidate: true}))
}

func serveFile(index *libraryIndex, selectPath func(library.Item) string, contentType string) http.HandlerFunc {
	return playback.LibraryFileHandler(playback.LibraryFileDependencies{Lookup: index.VisibleItem, Select: selectPath, Safe: index.Safe, NotFound: localizedNotFound, ContentType: contentType})
}
