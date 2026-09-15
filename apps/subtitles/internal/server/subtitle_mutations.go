package server

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (manager *subtitleManager) fetchWeb(writer http.ResponseWriter, request *http.Request) {
	if !emptyMutationRequest(writer, request) {
		localizedError(writer, request, "subtitle request is invalid", http.StatusBadRequest)
		return
	}
	item, found := visibleItem(request, manager.index, request.PathValue("id"))
	if !found || item.Kind != "video" {
		localizedNotFound(writer, request)
		return
	}
	_, _, missing := subtitleCoverageAll(item, manager.settings.subtitleLanguages())
	if len(missing) == 0 {
		localizedError(writer, request, "subtitle already exists", http.StatusConflict)
		return
	}
	language, searchable := manager.searchableSubtitleLanguage(request.Context(), item, missing)
	if !searchable {
		localizedError(writer, request, "subtitle needs a local text track", http.StatusConflict)
		return
	}
	status, err := manager.fetch(request, item.ID, language)
	if err != nil {
		localizedError(writer, request, "subtitle could not be added", status)
		return
	}
	http.Redirect(writer, request, "/?view=wanted", http.StatusSeeOther)
}

func (manager *subtitleManager) fetchWantedWeb(writer http.ResponseWriter, request *http.Request) {
	if !emptyMutationRequest(writer, request) {
		localizedError(writer, request, "subtitle request is invalid", http.StatusBadRequest)
		return
	}
	_, written, err := manager.fetchWantedLanguages(request, manager.settings.subtitleLanguages(), 10)
	if err != nil && written == 0 {
		localizedError(writer, request, "subtitles could not be added", http.StatusBadGateway)
		return
	}
	http.Redirect(writer, request, "/?view=wanted", http.StatusSeeOther)
}

func (manager *subtitleManager) maintainWeb(writer http.ResponseWriter, request *http.Request) {
	if !emptyMutationRequest(writer, request) {
		localizedError(writer, request, "subtitle request is invalid", http.StatusBadRequest)
		return
	}
	result, err := manager.maintainLanguages(request, manager.settings.subtitleLanguages(), 10)
	if err != nil && result.Added+result.Upgraded == 0 {
		localizedError(writer, request, "subtitles could not be maintained", http.StatusBadGateway)
		return
	}
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (manager *subtitleManager) restoreWeb(writer http.ResponseWriter, request *http.Request) {
	if !emptyMutationRequest(writer, request) {
		localizedError(writer, request, "subtitle restore request is invalid", http.StatusBadRequest)
		return
	}
	if status, err := manager.restore(request, request.PathValue("id")); err != nil {
		localizedError(writer, request, err.Error(), status)
		return
	}
	http.Redirect(writer, request, "/?view=library", http.StatusSeeOther)
}

func (manager *subtitleManager) replacementWeb(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "replaceable") || len(request.PostForm["replaceable"]) != 1 || !oneOf(request.PostForm.Get("replaceable"), "true", "false") {
		localizedError(writer, request, "subtitle replacement request is invalid", http.StatusBadRequest)
		return
	}
	if status, err := manager.setReplacement(request, request.PathValue("id"), request.PostForm.Get("replaceable") == "true"); err != nil {
		localizedError(writer, request, err.Error(), status)
		return
	}
	http.Redirect(writer, request, "/?view=library", http.StatusSeeOther)
}

func (manager *subtitleManager) testProvidersWeb(writer http.ResponseWriter, request *http.Request) {
	if !emptyMutationRequest(writer, request) {
		localizedError(writer, request, "provider test request is invalid", http.StatusBadRequest)
		return
	}
	manager.provider.testCredentials(request.Context())
	http.Redirect(writer, request, "/settings#provider", http.StatusSeeOther)
}

func (manager *subtitleManager) statusAPI(writer http.ResponseWriter, request *http.Request) {
	options, err := subtitleDashboardQuery(request)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	data, err := manager.projection(request, options)
	if err != nil {
		apiError(writer, errors.New("subtitle library is unavailable"), http.StatusServiceUnavailable)
		return
	}
	writeJSON(writer, data, http.StatusOK)
}

func (manager *subtitleManager) fetchAPI(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Language json.RawMessage `json:"language"`
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	language := manager.settings.subtitleLanguage()
	if input.Language != nil {
		if !decodeSubtitleValue(input.Language, &language) {
			apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
			return
		}
	}
	status, err := manager.fetch(request, request.PathValue("id"), language)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writer.WriteHeader(http.StatusCreated)
}

func (manager *subtitleManager) fetchWantedAPI(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Missing fields receive defaults while explicit malformed values are rejected independently.
	var input struct {
		Language json.RawMessage `json:"language"`
		Limit    json.RawMessage `json:"limit"`
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	languages, limit := manager.settings.subtitleLanguages(), 10
	if input.Language != nil {
		var language string
		if !decodeSubtitleValue(input.Language, &language) {
			apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
			return
		}
		languages = []string{language}
	}
	if input.Limit != nil {
		if !decodeSubtitleValue(input.Limit, &limit) {
			apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
			return
		}
	}
	if _, err := validateSubtitleLanguages(languages); err != nil || limit < 1 || limit > 50 {
		apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
		return
	}
	attempted, written, err := manager.fetchWantedLanguages(request, languages, limit)
	if err != nil && written == 0 {
		apiError(writer, errors.New("subtitle provider unavailable"), http.StatusBadGateway)
		return
	}
	writeJSON(writer, map[string]int{"attempted": attempted, "written": written, "failed": attempted - written}, http.StatusOK)
}

func (manager *subtitleManager) maintainAPI(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // The API validates one bounded request before the shared maintenance operation.
	var input struct {
		Language json.RawMessage `json:"language"`
		Limit    json.RawMessage `json:"limit"`
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	languages, limit := manager.settings.subtitleLanguages(), 10
	if input.Language != nil {
		var language string
		if !decodeSubtitleValue(input.Language, &language) {
			apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
			return
		}
		languages = []string{language}
	}
	if input.Limit != nil && !decodeSubtitleValue(input.Limit, &limit) {
		apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
		return
	}
	if _, err := validateSubtitleLanguages(languages); err != nil || limit < 1 || limit > 50 {
		apiError(writer, errors.New("subtitle request is invalid"), http.StatusBadRequest)
		return
	}
	result, err := manager.maintainLanguages(request, languages, limit)
	if err != nil && result.Added+result.Upgraded == 0 {
		apiError(writer, errors.New("subtitle maintenance failed"), http.StatusBadGateway)
		return
	}
	writeJSON(writer, result, http.StatusOK)
}

func (manager *subtitleManager) restoreAPI(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Language json.RawMessage `json:"language"`
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	language := manager.settings.subtitleLanguage()
	if input.Language != nil && !decodeSubtitleValue(input.Language, &language) {
		apiError(writer, errors.New("subtitle language is invalid"), http.StatusBadRequest)
		return
	}
	if status, err := manager.restoreLanguage(request, request.PathValue("id"), language); err != nil {
		apiError(writer, err, status)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (manager *subtitleManager) replacementAPI(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Replaceable json.RawMessage `json:"replaceable"`
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	var replaceable bool
	if input.Replaceable == nil || !decodeSubtitleValue(input.Replaceable, &replaceable) {
		apiError(writer, errors.New("subtitle replacement request is invalid"), http.StatusBadRequest)
		return
	}
	if status, err := manager.setReplacement(request, request.PathValue("id"), replaceable); err != nil {
		apiError(writer, err, status)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (manager *subtitleManager) testProvidersAPI(writer http.ResponseWriter, request *http.Request) {
	if !readSubtitleJSON(writer, request, &struct{}{}) {
		return
	}
	attempted, connected := manager.provider.testCredentials(request.Context())
	writeJSON(writer, map[string]any{"attempted": attempted, "connected": connected, "providers": manager.provider.healthViews()}, http.StatusOK)
}
