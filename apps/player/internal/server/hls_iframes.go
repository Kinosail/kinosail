package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

func newHLSIFrames(cache, ffmpeg string, workloads *workload.Governor) *playback.IFrames {
	return playback.NewIFrames(cache, ffmpeg, func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Interactive) })
}

func (manager *hlsManager) iframeHeight(ctx context.Context, item library.Item) int {
	video := mediaFactsFor(item, manager.probe.facts(ctx, item)).Video
	return playback.IFrameHeight(video.Width, video.Height, video.Rotation)
}

func (manager *hlsManager) iframeProjection(request *http.Request, item library.Item, recipe hlsRecipe, name string, duration float64, projection func([]byte) []byte) func([]byte) []byte {
	if name != "index.m3u8" || item.Kind != "video" || recipe.offset != 0 || !strings.HasPrefix(request.PathValue("file"), "p/") || jellyfinPlaySessionQuery(request) != "" {
		return projection
	}
	if _, cast := request.Context().Value(castTicketKey{}).(string); cast {
		return projection
	}
	height := manager.iframeHeight(request.Context(), item)
	if height == 0 {
		return projection
	}
	playbackDuration := hlsPlaybackDuration(recipe, duration)
	return func(manifest []byte) []byte {
		manifest = projectHLSPlaylist(manifest, playbackDuration, projection)
		if manifest == nil {
			return nil
		}
		return playback.WithIFrameStream(manifest, height)
	}
}

func (manager *hlsManager) serveIFrames(writer http.ResponseWriter, request *http.Request) {
	item, found := manager.hlsLookup(request, request.PathValue("id"))
	recipe, err := playback.ParseHLSRecipe(request.PathValue("recipe"), hlsPolicy())
	if !found || err != nil || item.Kind != "video" {
		localizedNotFound(writer, request)
		return
	}
	if !hlsAllowed(request) {
		hlsForbidden(writer, request)
		return
	}
	err = manager.iframes.Serve(writer, request, playback.IFrameRequest{
		ID: item.ID, Source: item.Path, File: request.PathValue("file"), Recipe: recipe,
		Duration: manager.hlsDuration(request, item), Height: manager.iframeHeight(request.Context(), item),
	})
	if err == nil || request.Context().Err() != nil {
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		localizedError(writer, request, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	slog.DebugContext(request.Context(), "I-frame request rejected", "request_id", requestActivityID(request.Context()), "item_id", item.ID)
	localizedNotFound(writer, request)
}
