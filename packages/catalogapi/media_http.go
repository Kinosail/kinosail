package catalogapi

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// MediaIndex exposes Viewer-filtered Library reads.
type MediaIndex interface {
	VisibleLibrary(*http.Request) []library.Item
	VisibleItem(*http.Request, string) (library.Item, bool)
}

// MediaProgress owns per-Viewer progress and public item projection.
type MediaProgress interface {
	ClientItems(*http.Request, []library.Item) any
	ClientItem(*http.Request, library.Item) any
	Dismiss(*http.Request, string) error
	Get(*http.Request, string) catalog.PlaybackState
	History(*http.Request, []library.Item) []library.Item
	SetRevision(*http.Request, string, float64, *bool, string, uint64) (bool, error)
}

// MediaLists owns per-Viewer lists, playlists, and Collections.
type MediaLists interface {
	catalog.CollectionStore
	Create(context.Context, string, string, ...string) error
	CreateSmart(context.Context, string, string, catalog.PlaylistRule) error
	DeletePlaylist(context.Context, string, string) error
	ExportPlaylistDocument(string, string) (PlaylistDocument, error)
	Has(*http.Request, string) bool
	Order(context.Context, string, string, []string) error
	Playlist(*http.Request, string, []library.Item) []library.Item
	PlaylistNames(*http.Request) []string
	PlaylistSummariesJSON(*http.Request, []library.Item, string) any
	SetListed(context.Context, string, string, bool) error
	SetPlaylist(context.Context, string, string, string, bool) error
}

// MediaHandlers owns the Player-canonical catalog HTTP interface.
type MediaHandlers struct {
	Index       MediaIndex
	Progress    MediaProgress
	Lists       MediaLists
	Viewer      func(*http.Request) string
	Timeline    func(string) (playback.Timeline, error)
	StoreStatus func(error) int
}

// NewMediaHandlers binds the canonical catalog API to app-owned state and authorization.
func NewMediaHandlers(index MediaIndex, progress MediaProgress, lists MediaLists, viewer func(*http.Request) string, timeline func(string) (playback.Timeline, error), storeStatus func(error) int) MediaHandlers {
	return MediaHandlers{Index: index, Progress: progress, Lists: lists, Viewer: viewer, Timeline: timeline, StoreStatus: storeStatus}
}

// MediaExtras keeps app-only handlers and owner authorization at the adapter.
type MediaExtras struct {
	PlaybackEvents http.HandlerFunc
	Reader         http.HandlerFunc
	ReaderProgress http.HandlerFunc
	Owner          func(http.Handler) http.Handler
}

// Register installs the shared catalog routes and app-only route adapters.
func (handlers MediaHandlers) Register(mux *http.ServeMux, extras MediaExtras) {
	mux.HandleFunc("GET /api/v1/items/{id}", handlers.item)
	mux.HandleFunc("PUT /api/v1/items/{id}/progress", handlers.progress)
	mux.HandleFunc("POST /api/v1/items/{id}/playback-events", extras.PlaybackEvents)
	mux.HandleFunc("DELETE /api/v1/items/{id}/continue-watching", handlers.dismissProgress)
	mux.HandleFunc("GET /api/v1/history", handlers.history)
	mux.HandleFunc("GET /api/v1/audio/{id}/queue", handlers.audioQueue)
	mux.HandleFunc("GET /api/v1/books/{id}/reader", extras.Reader)
	mux.HandleFunc("GET /api/v1/books/{id}/reader/progress", extras.ReaderProgress)
	mux.HandleFunc("PUT /api/v1/books/{id}/reader/progress", extras.ReaderProgress)
	mux.HandleFunc("PUT /api/v1/items/{id}/list", handlers.list)
	mux.HandleFunc("GET /api/v1/playlists", handlers.playlists)
	mux.HandleFunc("POST /api/v1/playlists", handlers.createPlaylist)
	mux.HandleFunc("POST /api/v1/smart-playlists", handlers.createSmartPlaylist)
	mux.HandleFunc("GET /api/v1/playlists/{name}", handlers.playlist)
	mux.HandleFunc("DELETE /api/v1/playlists/{name}", handlers.deletePlaylist)
	mux.HandleFunc("PUT /api/v1/playlists/{name}/items/{id}", handlers.savePlaylist)
	mux.HandleFunc("PUT /api/v1/playlists/{name}/order", handlers.orderPlaylist)
	handlers.registerBrowse(mux)
	collections := catalog.CollectionHandlers{Index: handlers.Index, Progress: handlers.Progress, Lists: handlers.Lists}
	mux.HandleFunc("GET /api/v1/collections", collections.List)
	mux.Handle("POST /api/v1/collections", extras.Owner(http.HandlerFunc(collections.Create)))
	mux.HandleFunc("GET /api/v1/collections/{name}", collections.Get)
	mux.Handle("DELETE /api/v1/collections/{name}", extras.Owner(http.HandlerFunc(collections.Delete)))
	mux.Handle("PUT /api/v1/collections/{name}/items/{id}", extras.Owner(http.HandlerFunc(collections.Save)))
}
