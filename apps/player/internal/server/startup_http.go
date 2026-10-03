package server

import (
	"errors"
	"net/http"
	"strings"

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
	value := startupRequest{request: request, item: item, viewer: currentViewer(request).ID}
	value.direct = input.Source == "/media/"+item.ID
	value.key = "direct:" + item.ID
	if !value.direct {
		prefix := "/hls/" + item.ID + "/"
		recipe, file, valid := plannedHLSFile(strings.TrimPrefix(input.Source, prefix))
		if len(input.Source) > 2048 || !strings.HasPrefix(input.Source, prefix) || !valid || file != "index.m3u8" {
			apiError(writer, errors.New("playback preparation source is invalid"), http.StatusBadRequest)
			return
		}
		if !hlsAllowed(request) {
			hlsForbidden(writer, request)
			return
		}
		facts := mediaFactsFor(item, api.probe.facts(request.Context(), item))
		resolved, err := playback.ResolveHLSSource(sharedHLSRecipe(recipe), facts, item.Subtitles)
		if err != nil || recipe.offset > 0 && !validHLSOffset(recipe.offset, facts.Duration) {
			apiError(writer, errors.New("playback preparation recipe is invalid"), http.StatusBadRequest)
			return
		}
		value.recipe = localHLSRecipe(resolved)
		value.key = hlsRecipeKey(item.ID, value.recipe)
	}
	if api.hls.cache == "" && !value.direct {
		apiError(writer, errors.New("compatible playback is not configured"), http.StatusServiceUnavailable)
		return
	}
	state := api.hls.startup.enqueue(value)
	status := http.StatusAccepted
	if state == "busy" {
		status = http.StatusTooManyRequests
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]string{"state": state}, status)
}
