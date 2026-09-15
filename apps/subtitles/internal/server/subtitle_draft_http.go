package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

func (manager *subtitleManager) subtitleDraftAPI(writer http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || request.URL.RawQuery != "" {
		apiError(writer, errors.New("local draft request must be JSON without query parameters"), http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1024))
	var input subtitleDraftInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err != nil || !validSubtitleEditJSON(data) || decoder.Decode(&input) != nil {
		apiError(writer, errors.New("local draft JSON is invalid"), http.StatusBadRequest)
		return
	}
	draft, status, err := manager.localSubtitleDraft(request, request.PathValue("id"), input)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writeJSON(writer, draft, status)
}

func (manager *subtitleManager) inspectSubtitleDraftAPI(writer http.ResponseWriter, request *http.Request) {
	language, _, err := manager.reviewQuery(request, false)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	item, status, err := manager.subtitleItem(request, request.PathValue("id"))
	if err != nil {
		apiError(writer, err, status)
		return
	}
	manager.drafts.Lock()
	draft := manager.drafts.current
	manager.drafts.Unlock()
	if draft.Item != item.ID || draft.Language != language {
		writeJSON(writer, subtitleDraft{State: "none", Message: "No local draft for this video and language."}, http.StatusOK)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, draft, http.StatusOK)
}
