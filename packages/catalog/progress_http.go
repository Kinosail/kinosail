package catalog

import (
	"math"
	"net/http"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type progressItemIndex interface {
	VisibleItem(*http.Request, string) (library.Item, bool)
}

// ProgressHTTPHandlers applies shared progress validation at each app's HTTP boundary.
type ProgressHTTPHandlers struct {
	Store    *RequestProgressStore
	Index    progressItemIndex
	Timeline func(string) (playback.Timeline, error)
	Error    func(http.ResponseWriter, *http.Request, string, int)
	NotFound func(http.ResponseWriter, *http.Request)
	Status   func(error) int
}

// NewProgressHTTPHandlers binds application adapters to the shared progress boundary.
func NewProgressHTTPHandlers(store *RequestProgressStore, index progressItemIndex, timeline func(string) (playback.Timeline, error), writeError func(http.ResponseWriter, *http.Request, string, int), notFound func(http.ResponseWriter, *http.Request), status func(error) int) ProgressHTTPHandlers {
	return ProgressHTTPHandlers{Store: store, Index: index, Timeline: timeline, Error: writeError, NotFound: notFound, Status: status}
}

// Save validates and stores a playback position.
func (handlers ProgressHTTPHandlers) Save() http.HandlerFunc { //nolint:cyclop // One boundary validates the complete progress form before its single state change.
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
		id := item.ID
		seconds, err := strconv.ParseFloat(request.FormValue("seconds"), 64)
		if !found || err != nil || seconds < 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
			handlers.Error(writer, request, "invalid progress", http.StatusBadRequest)
			return
		}
		timeline, err := handlers.Timeline(request.URL.Query().Get("playbackToken"))
		if err != nil {
			handlers.Error(writer, request, "invalid progress", http.StatusBadRequest)
			return
		}
		if len(timeline.Omitted) > 0 {
			seconds = timeline.SourceTime(seconds)
		}
		watched, valid := watchedValue(request.FormValue("watched"))
		if !valid {
			handlers.Error(writer, request, "invalid watched state", http.StatusBadRequest)
			return
		}
		revision, valid := revisionValue(request.FormValue("revision"))
		if !valid {
			handlers.Error(writer, request, "invalid progress revision", http.StatusBadRequest)
			return
		}
		if _, err := handlers.Store.SetRevision(request, id, seconds, watched, request.FormValue("session"), revision); err != nil { //nolint:contextcheck // Validated atomic progress commits must finish after client cancellation.
			handlers.Error(writer, request, err.Error(), handlers.Status(err))
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

// SaveWatched validates and stores the watched state submitted by the web UI.
func (handlers ProgressHTTPHandlers) SaveWatched() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
		id := item.ID
		watched, err := strconv.ParseBool(request.FormValue("watched"))
		if !found || err != nil {
			handlers.Error(writer, request, "invalid watched state", http.StatusBadRequest)
			return
		}
		if err := handlers.Store.Set(request, id, 0, &watched); err != nil { //nolint:contextcheck // Validated atomic progress commits must finish after client cancellation.
			handlers.Error(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/watch/"+id, http.StatusSeeOther)
	}
}

// Dismiss removes visible progress from continue watching.
func (handlers ProgressHTTPHandlers) Dismiss() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := handlers.Index.VisibleItem(request, request.PathValue("id"))
		id := item.ID
		if !found {
			handlers.NotFound(writer, request)
			return
		}
		if err := handlers.Store.Dismiss(request, id); err != nil { //nolint:contextcheck // Validated atomic progress commits must finish after client cancellation.
			handlers.Error(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}

func watchedValue(raw string) (*bool, bool) {
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseBool(raw)
	return &value, err == nil
}

func revisionValue(raw string) (uint64, bool) {
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	return value, err == nil
}
