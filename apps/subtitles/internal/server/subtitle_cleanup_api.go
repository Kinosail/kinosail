package server

import (
	"errors"
	"net/http"
	"slices"
)

type subtitleCleanupAPIInput struct {
	Enabled   bool     `json:"enabled"`
	Languages []string `json:"languages"`
	Forced    string   `json:"forced"`
	Digest    string   `json:"digest,omitempty"`
}

func readSubtitleCleanupAPI(writer http.ResponseWriter, request *http.Request, applying bool) (subtitleCleanupAPIInput, bool) { //nolint:cyclop // Keep strict encoding, opt-in, policy, and digest checks at the API boundary.
	var input subtitleCleanupAPIInput
	if request.URL.RawQuery != "" || request.Header.Get("Content-Type") != "application/json" {
		apiError(writer, errors.New("subtitle cleanup request is invalid"), http.StatusBadRequest)
		return input, false
	}
	if !readSubtitleJSON(writer, request, &input) {
		return input, false
	}
	if !input.Enabled || len(input.Languages) == 0 || len(input.Languages) > maximumSubtitleLanguages || !oneOf(input.Forced, "keep", "delete") || (!applying && input.Digest != "") || (applying && !validSubtitleCleanupDigest(input.Digest)) {
		apiError(writer, errors.New("subtitle cleanup request is invalid"), http.StatusBadRequest)
		return input, false
	}
	canonical, err := validateSubtitleLanguages(input.Languages)
	if err != nil {
		apiError(writer, errors.New("subtitle cleanup request is invalid"), http.StatusBadRequest)
		return input, false
	}
	input.Languages = canonical
	return input, true
}

func apiPreviewSubtitleCleanup(index *libraryIndex, settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		input, ok := readSubtitleCleanupAPI(writer, request, false)
		if !ok {
			return
		}
		if settings == nil || !slices.Equal(settings.subtitleLanguages(), input.Languages) && settings.editable("subtitles.language") != nil {
			apiError(writer, errors.New("preferred subtitle languages are managed by deployment configuration"), http.StatusConflict)
			return
		}
		plan, err := planSubtitleCleanup(index, input.Languages, input.Forced)
		if err != nil {
			apiError(writer, errors.New("subtitle cleanup preview is unavailable"), http.StatusServiceUnavailable)
			return
		}
		files := make([]map[string]string, 0, min(len(plan.Files), 100))
		for _, file := range plan.Files[:min(len(plan.Files), 100)] {
			files = append(files, map[string]string{"path": file.Path})
		}
		writeJSON(writer, map[string]any{"languages": plan.Languages, "forced": input.Forced, "digest": plan.Digest, "count": len(plan.Files), "skipped": plan.Skipped, "files": files}, http.StatusOK)
	}
}

func apiApplySubtitleCleanup(index *libraryIndex, settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		input, ok := readSubtitleCleanupAPI(writer, request, true)
		if !ok {
			return
		}
		removed, err := applySubtitleCleanup(index, settings, input.Languages, input.Forced, input.Digest)
		if err != nil {
			apiError(writer, errors.New("subtitle cleanup stopped; preview again"), http.StatusConflict)
			return
		}
		writeJSON(writer, map[string]any{"languages": input.Languages, "removed": removed}, http.StatusOK)
	}
}
