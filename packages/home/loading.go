package home

import (
	"log/slog"
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

func renderLoading[Show, Resume, Playlist, Collection any](source Source[Show, Resume, Playlist, Collection], view View, tmdb bool, writer http.ResponseWriter, request *http.Request) {
	shell := source.Shell(request, "all")
	data := Page[Show, Resume, Playlist, Collection]{
		ServerName: shell.ServerName, Owner: shell.Owner, ViewerName: shell.ViewerName, ViewerID: shell.ViewerID, CanLogout: shell.CanLogout,
		View: "all", TMDB: tmdb, NavigationPrimary: shell.NavigationPrimary, NavigationMore: shell.NavigationMore, UpdateAvailable: shell.UpdateAvailable,
		CommandMenu: true, Loading: true, RetryAfter: library.LoadingRetrySeconds,
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	library.MarkLoadingRetry(writer.Header(), library.ErrLoading)
	writer.WriteHeader(http.StatusServiceUnavailable)
	if err := view.Execute(writer, request, data); err != nil {
		slog.Error("render home", "operation", "library.loading", "error", err)
	}
}
