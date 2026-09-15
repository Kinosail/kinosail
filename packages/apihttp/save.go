package apihttp

import (
	"context"
	"errors"
	"net/http"
)

var errSaveUnavailable = errors.New("save unavailable")

// Save decodes one request and returns Player's canonical saved response.
func Save[T any](save func(T) error, conflictErrors ...error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if save == nil {
			Error(writer, errSaveUnavailable, http.StatusInternalServerError)
			return
		}
		var input T
		if !ReadJSON(writer, request, &input) {
			return
		}
		if err := save(input); err != nil {
			Error(writer, err, saveStatus(err, conflictErrors))
			return
		}
		WriteJSON(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}

func saveStatus(err error, conflictErrors []error) int {
	for _, conflict := range conflictErrors {
		if conflict != nil && errors.Is(err, conflict) {
			return http.StatusConflict
		}
	}
	return http.StatusBadRequest
}

// SaveLibraries applies one library path change and returns the refreshed list.
func SaveLibraries(change func(string) error, refresh func(context.Context) (any, error)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if change == nil || refresh == nil {
			Error(writer, errSaveUnavailable, http.StatusInternalServerError)
			return
		}
		var input struct{ Path string }
		if !ReadJSON(writer, request, &input) {
			return
		}
		if err := change(input.Path); err != nil {
			Error(writer, err, http.StatusBadRequest)
			return
		}
		libraries, err := refresh(request.Context())
		if err != nil {
			Error(writer, err, http.StatusInternalServerError)
			return
		}
		WriteJSON(writer, map[string]any{"libraries": libraries}, http.StatusOK)
	}
}
