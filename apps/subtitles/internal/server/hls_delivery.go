package server

import (
	"context"
	"errors"
	"log/slog"
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
			logHLSPlanRejection(request, err)
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
	root, plan, err := manager.openHLSGeneration(request, item, recipe, key)
	if err != nil {
		logHLSPlanRejection(request, err)
		localizedNotFound(writer, request)
		return
	}
	defer root.Close()

	duration := 0.0
	if filepath.Base(name) == "index.m3u8" {
		duration = playback.HLSPlaybackDuration(sharedHLSRecipe(recipe), manager.probe.duration(request.Context(), item))
		if err := manager.waitForHLSProjection(request, root, key, name, duration, plan); err != nil {
			localizedError(writer, request, "playlist timeline is not ready", http.StatusServiceUnavailable)
			return
		}
	}
	file, err := manager.openHLSDeliveryFile(request, root, item, recipe, key, name, plan)
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	defer file.Close()
	if plan != nil {
		if err := manager.validateHLSPolicy(request.Context(), item, recipe, plan.Policy); err != nil {
			logHLSPlanRejection(request, err)
			localizedNotFound(writer, request)
			return
		}
	}
	if err := plan.validateRendition(root, name); err != nil {
		logHLSPlanRejection(request, errCopiedHLSPlan)
		localizedNotFound(writer, request)
		return
	}
	if filepath.Ext(name) == ".m3u8" {
		serveHLSPlaylistWithSession(writer, request, file, duration, plan)
		return
	}
	info, err := file.Stat()
	if err != nil {
		localizedNotFound(writer, request)
		return
	}
	http.ServeContent(writer, request, name, info.ModTime(), file)
}

func (manager *hlsManager) openHLSGeneration(request *http.Request, item library.Item, recipe hlsRecipe, key string) (*os.Root, *copiedHLSPlan, error) {
	cache, err := os.OpenRoot(manager.cache)
	if err != nil {
		return nil, nil, err
	}
	defer cache.Close()
	root, err := cache.OpenRoot(key)
	if err != nil {
		return nil, nil, err
	}
	if !copiedHLSMode(recipe) {
		return root, nil, nil
	}
	options, err := manager.hlsSettings(item, recipe)
	var plan *copiedHLSPlan
	if err == nil {
		plan, err = loadCopiedHLSPlan(root, options.Cache)
	}
	if err == nil {
		err = manager.validateHLSPolicy(request.Context(), item, recipe, options.Cache)
	}
	if err != nil {
		_ = root.Close()
		return nil, nil, err
	}
	return root, plan, nil
}

func logHLSPlanRejection(request *http.Request, err error) {
	outcome := ""
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "readiness-timeout"
	case errors.Is(err, errCopiedHLSPlan):
		outcome = "plan-invalid"
	case errors.Is(err, errHLSIdentityChanged), errors.Is(err, playback.ErrHLSSourceChanged):
		outcome = "identity-changed"
	}
	if outcome != "" {
		slog.WarnContext(request.Context(), "HLS generation rejected", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "outcome", outcome)
	}
}
