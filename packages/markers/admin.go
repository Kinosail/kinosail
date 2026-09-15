package markers

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

// Admin adapts marker operations to one application's routing and response policy.
type Admin struct {
	Analyzer *Analyzer
	Owner    func(http.Handler) http.Handler
	Find     func(*http.Request, string) (library.Item, bool)
	Snapshot func() ([]library.Item, error)
	Duration func(*http.Request, library.Item) float64
	JSON     func(http.ResponseWriter, any, int)
	Error    func(http.ResponseWriter, *http.Request, string, int)
	NotFound func(http.ResponseWriter, *http.Request)
}

// RegisterAdmin registers the owner-only marker management contract.
func RegisterAdmin(mux *http.ServeMux, admin Admin) {
	save := admin.Owner(http.HandlerFunc(admin.saveManual))
	remove := admin.Owner(http.HandlerFunc(admin.removeManual))
	mux.Handle("POST /markers/{id}", save)
	mux.Handle("POST /markers/{id}/remove", remove)
	mux.Handle("PUT /api/v1/items/{id}/markers", save)
	mux.Handle("DELETE /api/v1/items/{id}/markers/{type}", remove)
	mux.Handle("GET /api/v1/marker-analysis", admin.Owner(http.HandlerFunc(admin.analysisStatus)))
	mux.Handle("POST /api/v1/marker-analysis", admin.Owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { admin.runAnalysis(writer, request, false) })))
	mux.Handle("POST /settings/marker-analysis", admin.Owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { admin.runAnalysis(writer, request, true) })))
}

func (admin Admin) analysisStatus(writer http.ResponseWriter, _ *http.Request) {
	state, items, message := admin.Analyzer.Status()
	admin.JSON(writer, map[string]any{"state": state, "items": items, "detectorVersion": DetectorVersion, "error": message}, http.StatusOK)
}

func (admin Admin) runAnalysis(writer http.ResponseWriter, request *http.Request, redirect bool) {
	items, err := admin.Snapshot()
	if err != nil {
		admin.Error(writer, request, "marker analysis is unavailable", http.StatusServiceUnavailable)
		return
	}
	admin.Analyzer.Enqueue(items)
	if redirect {
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
		return
	}
	admin.JSON(writer, map[string]string{"state": "queued"}, http.StatusAccepted)
}

func (admin Admin) saveManual(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !validMarkerItemID(id) {
		admin.NotFound(writer, request)
		return
	}
	item, found := admin.Find(request, id)
	if !found {
		admin.NotFound(writer, request)
		return
	}
	input, err := readManualMarker(writer, request)
	if err != nil {
		admin.Error(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err = admin.Analyzer.SetManual(item, input.Type, input.Start, input.End, admin.Duration(request, item)); err != nil {
		admin.markerError(writer, request, err)
		return
	}
	if request.Method == http.MethodPut {
		admin.JSON(writer, map[string]any{"type": input.Type, "start": input.Start, "end": input.End}, http.StatusOK)
		return
	}
	http.Redirect(writer, request, "/watch/"+item.ID, http.StatusSeeOther)
}

type manualMarkerInput struct {
	Type  string  `json:"type"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func readManualMarker(writer http.ResponseWriter, request *http.Request) (manualMarkerInput, error) { //nolint:cyclop // JSON and form transports share one bounded marker parser.
	request.Body = http.MaxBytesReader(writer, request.Body, 4<<10)
	if request.Method == http.MethodPut {
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" || request.URL.RawQuery != "" {
			return manualMarkerInput{}, errors.New("playback marker is invalid")
		}
		var input manualMarkerInput
		if err = httpguard.DecodeJSON(request.Body, 4<<10, &input, true); err != nil {
			return input, errors.New("playback marker is invalid")
		}
		return input, nil
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyMarkerFormKeys(request.PostForm, "type", "start", "end") {
		return manualMarkerInput{}, errors.New("playback marker is invalid")
	}
	markerType, typeOK := oneMarkerFormValue(request.PostForm, "type", 32)
	start, startOK := markerNumber(request.PostForm, "start")
	end, endOK := markerNumber(request.PostForm, "end")
	if !typeOK || !startOK || !endOK {
		return manualMarkerInput{}, errors.New("playback marker is invalid")
	}
	return manualMarkerInput{Type: markerType, Start: start, End: end}, nil
}

func (admin Admin) removeManual(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Both API and form routes enforce the same bounded removal operation.
	id := request.PathValue("id")
	if !validMarkerItemID(id) {
		admin.NotFound(writer, request)
		return
	}
	item, found := admin.Find(request, id)
	if !found {
		admin.NotFound(writer, request)
		return
	}
	markerType := request.PathValue("type")
	if request.Method != http.MethodDelete {
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<10)
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyMarkerFormKeys(request.PostForm, "type") {
			admin.Error(writer, request, "playback marker type is required", http.StatusBadRequest)
			return
		}
		markerType, found = oneMarkerFormValue(request.PostForm, "type", 32)
	}
	if !found {
		admin.Error(writer, request, "playback marker type is invalid", http.StatusBadRequest)
		return
	}
	if err := admin.Analyzer.Suppress(item, markerType); err != nil {
		admin.markerError(writer, request, err)
		return
	}
	if request.Method == http.MethodDelete {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(writer, request, "/watch/"+item.ID, http.StatusSeeOther)
}

func (admin Admin) markerError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, ErrPersistence) {
		status = http.StatusInternalServerError
	}
	admin.Error(writer, request, err.Error(), status)
}

func onlyMarkerFormKeys(form url.Values, keys ...string) bool {
	if len(form) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, found := form[key]; !found {
			return false
		}
	}
	return true
}

func oneMarkerFormValue(form url.Values, key string, maximum int) (string, bool) {
	values := form[key]
	if len(values) != 1 || values[0] == "" || len(values[0]) > maximum {
		return "", false
	}
	return values[0], true
}

func markerNumber(form url.Values, key string) (float64, bool) {
	value, ok := oneMarkerFormValue(form, key, 64)
	if !ok {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil
}

func validMarkerItemID(id string) bool {
	return id != "" && len(id) <= 256 && !strings.ContainsAny(id, `/\`) && strings.IndexFunc(id, unicode.IsControl) < 0
}
