package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func registerFiles(mux *http.ServeMux, index *libraryIndex, probe *mediaProbe) {
	mux.HandleFunc("GET /media/{id}", serveFile(index, playback.MediaPath, ""))
	mux.HandleFunc("GET /download/{id}", download(index))
	mux.HandleFunc("GET /subtitle/{id}", serveSubtitle(index))
	mux.HandleFunc("GET /subtitle/{id}/{track}", serveSubtitle(index))
	mux.HandleFunc("GET /subtitle/{id}/embedded/{stream}", probe.serveEmbedded(index))
	mux.HandleFunc("GET /art/{id}", serveArtwork(index))
	mux.HandleFunc("GET /backdrop/{id}", serveFile(index, playback.BackdropPath, ""))
	mux.HandleFunc("GET /person/{id}/{person}", playback.LibraryPersonHandler(index.VisibleItem))
}

func serveArtwork(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		variant := query["variant"]
		if len(query) > 1 || len(query) == 1 && (len(variant) != 1 || variant[0] != "episode") {
			localizedNotFound(writer, request)
			return
		}
		selectPath := playback.ArtworkPath
		if len(variant) == 1 {
			selectPath = episodeArtworkPath
		}
		serveFile(index, selectPath, "")(writer, request)
	}
}

func download(index *libraryIndex) http.HandlerFunc {
	return playback.LibraryDownloadHandler(playback.LibraryDownloadDependencies{Allowed: canDownload, Lookup: index.VisibleItem, NotFound: localizedNotFound, Error: localizedError})
}

func canDownload(request *http.Request) bool {
	viewer := currentViewer(request)
	return viewer.Permits("download", viewer.Owner || viewer.Downloads)
}

func serveFile(index *libraryIndex, selectPath func(library.Item) string, contentType string) http.HandlerFunc {
	return playback.LibraryFileHandler(playback.LibraryFileDependencies{Lookup: index.VisibleItem, Select: selectPath, Safe: index.Safe, NotFound: localizedNotFound, ContentType: contentType})
}

func serveSubtitle(index *libraryIndex) http.HandlerFunc {
	return playback.LibrarySubtitleHandler(playback.LibrarySubtitleDependencies{Lookup: index.VisibleItem, Safe: index.Safe, Delivery: playback.SubtitleDeliveryDependencies{ParseTimeline: timelineFromPlaybackToken, NotFound: localizedNotFound, Error: localizedError}})
}

func writeSubtitle(writer http.ResponseWriter, request *http.Request, subtitle string, timelines ...PlaybackTimeline) {
	playback.ServeSubtitle(writer, request, subtitle, timelines, playback.SubtitleDeliveryDependencies{ParseTimeline: timelineFromPlaybackToken, NotFound: localizedNotFound, Error: localizedError})
}

func artworkPath(item library.Item) string {
	return playback.ArtworkPath(item)
}

func episodeArtworkPath(item library.Item) string { return item.Artwork }

func showBackdropID(show library.Show) string { return catalogapi.ShowBackdropID(show) }
