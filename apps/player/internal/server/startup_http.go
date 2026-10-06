package server

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (api apiServices) preparePlayback(writer http.ResponseWriter, request *http.Request) {
	item, found := visibleItem(request, api.index, request.PathValue("id"))
	if !found {
		apiNotFound(writer)
		return
	}
	if !currentViewer(request).Permits("stream", true) {
		apiError(writer, errors.New("playback is not allowed"), http.StatusForbidden)
		return
	}
	if request.URL.RawQuery != "" {
		apiError(writer, errors.New("playback preparation query is invalid"), http.StatusBadRequest)
		return
	}
	if request.Method == http.MethodDelete {
		api.hls.startup.cancelItem(currentViewer(request).ID, item.ID)
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	var input struct {
		Source string `json:"source"`
	}
	if !readJSON(writer, request, &input) {
		return
	}
	value, valid := api.startupSource(writer, request, item, input.Source)
	if !valid {
		return
	}
	if api.hls.cache == "" && !value.direct {
		apiError(writer, errors.New("compatible playback is not configured"), http.StatusServiceUnavailable)
		return
	}
	state := api.hls.startup.enqueue(request.Context(), value)
	status := http.StatusAccepted
	if state == "busy" {
		status = http.StatusTooManyRequests
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]string{"state": state}, status)
}

func (api apiServices) startupSource(writer http.ResponseWriter, request *http.Request, item library.Item, source string) (startupRequest, bool) {
	// Own the queued request before post-handler middleware can mutate its forms.
	value := startupRequest{request: request.Clone(request.Context()), item: item, viewer: currentViewer(request).ID, key: "direct:" + item.ID}
	value.direct = source == "/media/"+item.ID
	if value.direct {
		return value, true
	}
	prefix := "/hls/" + item.ID + "/"
	recipe, file, valid := plannedHLSFile(strings.TrimPrefix(source, prefix))
	if len(source) > 2048 || !strings.HasPrefix(source, prefix) || !valid || file != "index.m3u8" {
		apiError(writer, errors.New("playback preparation source is invalid"), http.StatusBadRequest)
		return value, false
	}
	if !hlsAllowed(request) {
		hlsForbidden(writer, request)
		return value, false
	}
	facts := mediaFactsFor(item, api.probe.facts(request.Context(), item))
	resolved, err := playback.ResolveHLSSource(sharedHLSRecipe(recipe), facts, item.Subtitles)
	if err != nil || recipe.offset > 0 && !validHLSOffset(recipe.offset, facts.Duration) {
		apiError(writer, errors.New("playback preparation recipe is invalid"), http.StatusBadRequest)
		return value, false
	}
	value.recipe = localHLSRecipe(resolved)
	value.key = hlsRecipeKey(item.ID, value.recipe)
	if api.hls.cache != "" {
		if _, err := api.hls.readHLSMasterRenditions(filepath.Join(api.hls.cache, value.key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.WarnContext(request.Context(), "HLS master rejected", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "failure_class", "invalid-master")
			apiNotFound(writer)
			return value, false
		}
	}
	return value, true
}
