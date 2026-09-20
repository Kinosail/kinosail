package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

func (manager *subtitleManager) registerSubtitleReview(mux *http.ServeMux, auth *authentication) {
	mux.Handle("GET /subtitles/inspect/{id}", auth.owner(http.HandlerFunc(manager.subtitleInspectorWeb)))
	mux.Handle("GET /api/v1/subtitle-library/{id}/inspect", auth.owner(http.HandlerFunc(manager.inspectSubtitleAPI)))
	mux.Handle("GET /api/v1/subtitle-library/{id}/export", auth.owner(http.HandlerFunc(manager.exportSubtitleAPI)))
	mux.Handle("GET /api/v1/subtitle-library/{id}/draft", auth.owner(http.HandlerFunc(manager.inspectSubtitleDraftAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/draft", auth.owner(http.HandlerFunc(manager.subtitleDraftAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/audio", auth.owner(http.HandlerFunc(manager.subtitleAudioAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/preview", auth.owner(http.HandlerFunc(manager.previewSubtitleAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/apply", auth.owner(http.HandlerFunc(manager.applySubtitleAPI)))
}

func (manager *subtitleManager) reviewQuery(request *http.Request, export bool) (string, string, error) {
	if len(request.URL.RawQuery) > 256 {
		return "", "", errors.New("subtitle request is invalid")
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return "", "", errors.New("subtitle request is invalid")
	}
	for key, values := range query {
		if len(values) != 1 || key != "language" && !(export && key == "format") {
			return "", "", errors.New("subtitle request is invalid")
		}
	}
	language := manager.settings.subtitleLanguage()
	if value, present := query["language"]; present {
		language = value[0]
	}
	languages, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		return "", "", errors.New("subtitle language is invalid")
	}
	format := "srt"
	if value, present := query["format"]; present {
		format = value[0]
	}
	if !oneOf(format, "srt", "vtt", "original") {
		return "", "", errors.New("subtitle format is invalid")
	}
	return languages[0], format, nil
}

func (manager *subtitleManager) inspectSubtitleAPI(writer http.ResponseWriter, request *http.Request) {
	language, _, err := manager.reviewQuery(request, false)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	review, status, err := manager.inspectSubtitle(request, request.PathValue("id"), language)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writeJSON(writer, review, status)
}

func (manager *subtitleManager) exportSubtitleAPI(writer http.ResponseWriter, request *http.Request) {
	language, format, err := manager.reviewQuery(request, true)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	data, mediaType, status, err := manager.exportSubtitle(request, request.PathValue("id"), language, format)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writer.Header().Set("Content-Type", mediaType)
	writer.Header().Set("Content-Disposition", `attachment; filename="`+subtitleOriginalName(format, data)+`"`)
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(data)
}

func (manager *subtitleManager) previewSubtitleAPI(writer http.ResponseWriter, request *http.Request) {
	var input subtitleEdit
	if !readSubtitleEdit(writer, request, &input) {
		return
	}
	review, _, status, err := manager.previewSubtitleEdit(request, request.PathValue("id"), input)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writeJSON(writer, review, status)
}

func (manager *subtitleManager) applySubtitleAPI(writer http.ResponseWriter, request *http.Request) {
	var input subtitleEdit
	if !readSubtitleEdit(writer, request, &input) {
		return
	}
	review, status, err := manager.applySubtitleEdit(request, request.PathValue("id"), input)
	if err != nil {
		apiError(writer, err, status)
		return
	}
	writeJSON(writer, review, status)
}

func readSubtitleEdit(writer http.ResponseWriter, request *http.Request, target *subtitleEdit) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || request.URL.RawQuery != "" {
		apiError(writer, errors.New("subtitle edit must be JSON without query parameters"), http.StatusBadRequest)
		return false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 10<<20)
	data, err := io.ReadAll(request.Body)
	if err != nil || !validSubtitleEditJSON(data) {
		apiError(writer, errors.New("subtitle edit JSON is invalid"), http.StatusBadRequest)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		apiError(writer, errors.New("subtitle edit JSON is invalid"), http.StatusBadRequest)
		return false
	}
	return true
}

// Validate all nested objects before decoding: duplicate fields and null values
// must not silently overwrite a language, anchor, or concurrency precondition.
func validSubtitleEditJSON(data []byte) bool {
	validator := subtitleEditJSONValidator{decoder: json.NewDecoder(bytes.NewReader(data))}
	if !validator.value(0) {
		return false
	}
	_, err := validator.decoder.Token()
	return errors.Is(err, io.EOF)
}

type subtitleEditJSONValidator struct {
	decoder *json.Decoder
	count   int
}

func (validator *subtitleEditJSONValidator) value(depth int) bool {
	validator.count++
	if depth > 4 || validator.count > 128 {
		return false
	}
	token, err := validator.decoder.Token()
	if err != nil || token == nil {
		return false
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return depth > 0
	}
	switch delimiter {
	case '{':
		return validator.object(depth)
	case '[':
		return validator.array(depth)
	default:
		return false
	}
}

func (validator *subtitleEditJSONValidator) object(depth int) bool {
	seen := map[string]bool{}
	for validator.decoder.More() {
		key, err := validator.decoder.Token()
		name, ok := key.(string)
		name = strings.ToLower(name)
		if err != nil || !ok || seen[name] {
			return false
		}
		seen[name] = true
		if !validator.value(depth + 1) {
			return false
		}
	}
	end, err := validator.decoder.Token()
	return err == nil && end == json.Delim('}')
}

func (validator *subtitleEditJSONValidator) array(depth int) bool {
	if depth == 0 {
		return false
	}
	for validator.decoder.More() {
		if !validator.value(depth + 1) {
			return false
		}
	}
	end, err := validator.decoder.Token()
	return err == nil && end == json.Delim(']')
}
