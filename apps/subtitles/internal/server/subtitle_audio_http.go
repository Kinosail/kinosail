package server

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (manager *subtitleManager) subtitleAudioAPI(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Language json.RawMessage `json:"language"`
	}
	if request.URL.RawQuery != "" {
		apiError(writer, errors.New("subtitle audio request is invalid"), http.StatusBadRequest)
		return
	}
	if !readSubtitleJSON(writer, request, &input) {
		return
	}
	language := manager.settings.subtitleLanguage()
	if input.Language != nil && !decodeSubtitleValue(input.Language, &language) {
		apiError(writer, errors.New("subtitle language is invalid"), http.StatusBadRequest)
		return
	}
	languages, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	item, status, err := manager.subtitleItem(request, request.PathValue("id"))
	if err != nil {
		apiError(writer, err, status)
		return
	}
	reference, err := manager.provider.sync.audioReference(request.Context(), item, languages[0])
	if err != nil {
		apiError(writer, errors.New("local audio analysis is unavailable for this video"), http.StatusConflict)
		return
	}
	writeJSON(writer, struct {
		Duration float64   `json:"duration"`
		Waveform []float64 `json:"waveform"`
		Speech   []float64 `json:"speech"`
	}{float64(len(reference.Speech)) * subtitleFrame.Seconds(), averageSubtitleFrames(reference.Waveform, max(1, (len(reference.Waveform)+1023)/1024)), averageSubtitleFrames(reference.Speech, max(1, (len(reference.Speech)+1023)/1024))}, http.StatusOK)
}
