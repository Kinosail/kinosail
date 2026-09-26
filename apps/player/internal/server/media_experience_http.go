package server

import (
	"errors"
	"net/http"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

func mediaPreferenceScope(item library.Item) string {
	if item.Show != "" {
		return "show:" + item.Show
	}
	return "item:" + item.ID
}

func registerMediaExperience(mux *http.ServeMux, store *mediaExperienceStore, index *libraryIndex, progress *progressStore) {
	mux.HandleFunc("GET /api/v1/me/media-preferences", store.defaultHandler())
	mux.HandleFunc("PUT /api/v1/me/media-preferences", store.defaultHandler())
	mux.HandleFunc("GET /api/v1/items/{id}/playback-preferences", store.itemHandler(index))
	mux.HandleFunc("PUT /api/v1/items/{id}/playback-preferences", store.itemHandler(index))
	mux.HandleFunc("DELETE /api/v1/items/{id}/playback-preferences", store.itemHandler(index))
	mux.HandleFunc("GET /api/v1/items/{id}/bookmarks", store.bookmarkHandler(index))
	mux.HandleFunc("POST /api/v1/items/{id}/bookmarks", store.bookmarkHandler(index))
	mux.HandleFunc("DELETE /api/v1/items/{id}/bookmarks/{bookmark}", store.bookmarkHandler(index))
	mux.HandleFunc("PUT /api/v1/items/{id}/progress/sync", syncMediaProgress(index, progress))
}

func (store *mediaExperienceStore) defaultHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var value mediaPreferences
		if request.Method == http.MethodPut {
			if !readJSON(writer, request, &value) {
				return
			}
			if err := value.validate(); err != nil {
				apiError(writer, err, http.StatusBadRequest)
				return
			}
		}
		viewer := currentViewer(request).ID
		store.mu.Lock()
		defer store.mu.Unlock()
		if request.Method == http.MethodPut {
			if err := store.change(func(next *mediaExperienceState) error {
				next.Defaults[experienceKey(viewer, "defaults")] = value
				return nil
			}); err != nil {
				apiError(writer, errMediaExperience, http.StatusInternalServerError)
				return
			}
		}
		writeJSON(writer, store.defaults(viewer), http.StatusOK)
	}
}

func (store *mediaExperienceStore) itemHandler(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store.itemHTTP(writer, request, index)
	}
}

func (store *mediaExperienceStore) itemHTTP(writer http.ResponseWriter, request *http.Request, index *libraryIndex) {
	item, found := visibleItem(request, index, request.PathValue("id"))
	if !found {
		apiNotFound(writer)
		return
	}
	var value playbackPreferences
	if request.Method == http.MethodPut {
		if !readJSON(writer, request, &value) {
			return
		}
		if err := value.validate(); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
	}
	viewer := currentViewer(request).ID
	key := experienceKey(viewer, mediaPreferenceScope(item))
	store.mu.Lock()
	defer store.mu.Unlock()
	if request.Method != http.MethodGet {
		if err := store.change(playbackOverrideChange(request.Method, key, value)); err != nil {
			apiError(writer, errMediaExperience, http.StatusInternalServerError)
			return
		}
	}
	value, overridden := store.value.Overrides[key]
	if !overridden {
		value = store.defaults(viewer).Playback
	}
	writeJSON(writer, map[string]any{"playback": value, "overridden": overridden}, http.StatusOK)
}

func (store *mediaExperienceStore) bookmarkHandler(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store.bookmarksHTTP(writer, request, index)
	}
}

func (store *mediaExperienceStore) bookmarksHTTP(writer http.ResponseWriter, request *http.Request, index *libraryIndex) {
	includeOffset, err := readerOffsetRequested(request)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	item, found := visibleItem(request, index, request.PathValue("id"))
	if !found {
		apiNotFound(writer)
		return
	}
	if !slices.Contains([]string{"video", "audio", "audiobook", "book"}, item.Kind) {
		apiError(writer, errors.New("this item does not support bookmarks"), http.StatusBadRequest)
		return
	}
	value, valid := readBookmarkRequest(writer, request, index, item)
	if !valid {
		return
	}
	bookmarkID := request.PathValue("bookmark")
	key := experienceKey(currentViewer(request).ID, "item:"+item.ID)
	store.mu.Lock()
	defer store.mu.Unlock()
	if request.Method != http.MethodGet {
		if err := store.change(bookmarkChange(request.Method, key, bookmarkID, value, store.bookmarks(key))); err != nil {
			writeBookmarkError(writer, err)
			return
		}
	}
	values := bookmarkResponse(store.bookmarks(key), includeOffset)
	status := http.StatusOK
	if request.Method == http.MethodPost {
		status = http.StatusCreated
	}
	writeJSON(writer, map[string]any{"bookmarks": values}, status)
}

var errBookmarkLimit = errors.New("bookmark limit reached")

func playbackOverrideChange(method, key string, value playbackPreferences) func(*mediaExperienceState) error {
	return func(next *mediaExperienceState) error {
		if method == http.MethodDelete {
			delete(next.Overrides, key)
		} else {
			next.Overrides[key] = value
		}
		return nil
	}
}

func writeBookmarkError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, errBookmarkLimit) {
		status = http.StatusBadRequest
	}
	apiError(writer, errMediaExperience, status)
}
