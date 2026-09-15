package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) serveRecipe(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name string) {
	key := hlsRecipeKey(item.ID, recipe)
	if filepath.Base(name) == "index.m3u8" {
		if err := manager.prepare(request.Context(), item, recipe); err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, playback.ErrHLSSource) {
				status = http.StatusBadRequest
			}
			localizedError(writer, request, err.Error(), status)
			return
		}
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		writer.Header().Set("Cache-Control", "no-store")
	}
	if manager.cache == "" {
		localizedNotFound(writer, request)
		return
	}
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	defer root.Close()
	duration := 0.0
	if filepath.Base(name) == "index.m3u8" {
		duration = playback.HLSPlaybackDuration(sharedHLSRecipe(recipe), manager.probe.duration(request.Context(), item))
		if err := manager.waitForHLSProjection(request, root, key, name, duration); err != nil {
			localizedError(writer, request, "playlist timeline is not ready", http.StatusServiceUnavailable)
			return
		}
	}
	file, err := manager.openHLSDeliveryFile(request, root, item, recipe, key, name)
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	defer file.Close()
	if filepath.Ext(name) == ".m3u8" {
		serveHLSPlaylistWithSession(writer, request, file, duration)
		return
	}
	info, err := file.Stat()
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	http.ServeContent(writer, request, name, info.ModTime(), file)
}
