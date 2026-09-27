package server

import (
	"net/http"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

func registerFiles(mux *http.ServeMux, index *libraryIndex, probe *mediaProbe, workloads *workload.Governor) {
	mux.HandleFunc("GET /media/{id}", serveFile(index, playback.MediaPath, ""))
	mux.HandleFunc("GET /download/{id}", download(index))
	mux.HandleFunc("GET /subtitle/{id}", serveSubtitle(index))
	mux.HandleFunc("GET /subtitle/{id}/{track}", serveSubtitle(index))
	mux.HandleFunc("GET /subtitle/{id}/embedded/{stream}", probe.serveEmbedded(index))
	mux.HandleFunc("GET /art/{id}", serveArtwork(index))
	mux.HandleFunc("GET /episode-art/{id}", serveEpisodeStill(index, probe, workloads))
	mux.HandleFunc("GET /backdrop/{id}", serveFile(index, playback.BackdropPath, ""))
	mux.HandleFunc("GET /person/{id}/{person}", playback.LibraryPersonHandler(index.VisibleItem))
}

func serveEpisodeStill(index *libraryIndex, probe *mediaProbe, workloads *workload.Governor) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			localizedNotFound(writer, request)
			return
		}
		item, found := index.VisibleItem(request, request.PathValue("id"))
		if !found || item.Kind != "video" || item.Show == "" || !index.Safe(item.Path) {
			localizedNotFound(writer, request)
			return
		}
		if probe.cacheDir == "" {
			localizedError(writer, request, "episode still unavailable", http.StatusServiceUnavailable)
			return
		}
		target := filepath.Join(probe.cacheDir, "episode-stills", item.ID+".jpg")
		if !playback.Fresh(target, item.Path) {
			if err := generateEpisodeStill(request, probe, workloads, item, target); err != nil {
				localizedError(writer, request, "episode still unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(writer, request, target) //nolint:gosec // The cache path uses a visible scanned item ID.
	}
}

func generateEpisodeStill(request *http.Request, probe *mediaProbe, workloads *workload.Governor, item library.Item, target string) error {
	release, err := workloads.Acquire(request.Context(), workload.Background)
	if err != nil {
		return err
	}
	defer release()
	if playback.Fresh(target, item.Path) {
		return nil
	}
	second := min(300, int(probe.core.Duration(request.Context(), item)*0.2))
	return playback.GenerateEpisodeStill(request.Context(), probe.cacheDir, probe.ffmpeg, item.Path, target, second)
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
