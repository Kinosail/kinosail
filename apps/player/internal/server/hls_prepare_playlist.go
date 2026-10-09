package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) adoptRecipeFile(request *http.Request, key, path string) {
	if request.Method == http.MethodGet {
		if _, err := os.Stat(path); err == nil {
			manager.startup.playback(key)
		}
	}
}

func (manager *hlsManager) prepareRecipePlaylist(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe) bool {
	prepareContext := context.WithValue(manager.ctx, requestActivityKey{}, &requestActivity{id: requestActivityID(request.Context()), playbackSession: requestPlaybackSession(request.Context())})
	prepareContext = context.WithValue(prepareContext, viewerContextKey{}, currentViewer(request))
	if request.Method != http.MethodGet {
		prepareContext = context.WithValue(prepareContext, startupMetadataKey{}, true)
	}
	prepareContext, cancel := context.WithTimeout(prepareContext, 30*time.Second)
	defer cancel()
	started := time.Now()
	cached := manager.startupWindowReady(prepareContext, item, recipe)
	if err := manager.prepare(prepareContext, item, recipe); err != nil { //nolint:contextcheck // Playlist preparation uses the Server lifecycle so a disconnected request does not destroy shared output.
		slog.ErrorContext(request.Context(), "HLS playlist preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "mode", recipe.mode, "encoder_phase", hlsReadinessPhase(err), "error", hlsDiagnostic(err, item.Path))
		status := http.StatusServiceUnavailable
		if errors.Is(err, playback.ErrHLSSource) {
			status = http.StatusBadRequest
		}
		localizedError(writer, request, err.Error(), status)
		return false
	}
	cacheState := "cold"
	if cached {
		cacheState = "warm"
	}
	writer.Header().Set("X-Kinosail-Startup-Cache", cacheState)
	slog.Info("HLS startup", "cache_state", cacheState, "prepare_ms", time.Since(started).Milliseconds())
	return true
}
