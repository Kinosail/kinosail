package server

import (
	"context"
	"log/slog"
	"math"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) copiedHLSVideo(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	if recipe.mode == "remux" {
		return true
	}
	return recipe.mode == "audio-transcode" && !recipe.dialogueBoost && !recipe.normalizeLoudness &&
		len(recipe.omitted) == 0 && item.Kind != "audio" && item.Kind != "audiobook" &&
		manager.probe.facts(ctx, item).Video.Codec == "h264"
}

func (manager *hlsManager) reusableCopiedHLS(ctx context.Context, item library.Item, directory, policy string, recipe hlsRecipe) bool {
	if ctx.Err() != nil || !cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, policy) && !seekCacheFresh(directory, item.Path, policy) {
		return false
	}
	if !manager.startupCompletionReusable(ctx, item, directory, policy) {
		return false
	}
	if !manager.copiedHLSVideo(ctx, item, recipe) || !manager.copiedHLSTimelinePresent(directory) {
		// Ordinary cold streams retain their pre-index cache and seek behavior.
		return true
	}
	timeline, err := manager.readCopiedHLSTimeline(directory, policy)
	if err != nil {
		slog.WarnContext(ctx, "HLS copied cache rejected", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", recipe.mode, "failure_class", "invalid-timeline")
		return false
	}
	return timeline.Clock != nil && ctx.Err() == nil
}

func (manager *hlsManager) bindCopiedHLSTimeline(ctx context.Context, directory string, startNumber int) error {
	preparation, ok := ctx.Value(startupEncodingKey{}).(*startupEncoding)
	if !ok || preparation.timeline == nil || startNumber != 0 {
		return nil
	}
	return manager.writeCopiedHLSTimeline(directory, preparation.timeline)
}

func copiedHLSSeekArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	if timeline == nil {
		return arguments, nil
	}
	if number < 0 || number >= len(timeline.Keys) {
		return nil, errCopiedHLSIndex
	}
	// FFmpeg can demux an earlier key for input -ss. Do not copy that preroll.
	arguments = append(arguments, "-copypriorss", "0")
	if number > 0 {
		if timeline.Clock == nil {
			return nil, errCopiedHLSIndex
		}
		start := timeline.point(number)
		floor := math.Floor(start*1_000_000) / 1_000_000
		offset := start - timeline.point(0) + *timeline.Clock - (start - floor)
		arguments = append(arguments, "-output_ts_offset", copiedHLSTime(offset))
		// The mux clock restores copied video DTS; audio already keeps its source
		// presentation offset. Avoid adding that clock twice at a refill boundary.
		clock := copiedHLSTime(*timeline.Clock)
		arguments = append(arguments, "-bsf:a", "setts=pts=PTS-"+clock+"/TB:dts=DTS-"+clock+"/TB")
	}
	return arguments, nil
}

func (manager *hlsManager) copiedPlaylistProjection(ctx context.Context, item library.Item, recipe hlsRecipe, directory, rendition, policy string) func([]byte) []byte {
	return func(manifest []byte) []byte {
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		timeline, err := manager.readCopiedHLSTimeline(directory, policy)
		if err != nil && manager.copiedHLSTimelinePresent(directory) {
			return nil
		}
		if err == nil {
			if result, valid := copiedHLSManifest(manifest, timeline); valid {
				return result
			}
		}
		if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
			// EOF correction reads generated media, not container format duration.
			manifest = manager.completedCopiedHLSProjection(ctx, filepath.Join(directory, rendition), policy, manifest)
		} else if err != nil {
			// Preserve the existing established-cadence projection for unindexed
			// cold caches; these cuts do not acquire an indexed-source certificate.
			manifest = completeHLSVOD(manifest, hlsPlaybackDuration(recipe, manager.probe.duration(ctx, item)))
		}
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		return manifest
	}
}

func (manager *hlsManager) copiedStartupProjection(item library.Item, recipe hlsRecipe, directory string) func([]byte) []byte {
	if manager.completeHEVCStartup(manager.ctx, item, recipe) {
		return func(manifest []byte) []byte {
			if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
				return manifest
			}
			return nil
		}
	}
	if !manager.copiedHLSVideo(manager.ctx, item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	timeline, err := manager.readCopiedHLSTimeline(directory, options.Cache)
	return func(manifest []byte) []byte {
		if err != nil && manager.copiedHLSTimelinePresent(directory) {
			return nil
		}
		if err == nil {
			projected, valid := copiedHLSManifest(manifest, timeline)
			if valid {
				return projected
			}
		}
		if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
			return manifest
		}
		return nil
	}
}

func projectHLSPlaylist(manifest []byte, duration float64, projection ...func([]byte) []byte) []byte {
	if len(projection) > 0 && projection[0] != nil {
		return projection[0](manifest)
	}
	return completeHLSVOD(manifest, duration)
}

func copiedHLSInputTime(value float64) string {
	return copiedHLSTime(math.Floor(value*1_000_000) / 1_000_000)
}

func (manager *hlsManager) recipePlaylistProjection(ctx context.Context, item library.Item, recipe hlsRecipe, key, name string) func([]byte) []byte {
	if filepath.Dir(name) == "." || filepath.Ext(name) != ".m3u8" || !manager.copiedHLSVideo(ctx, item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	return manager.copiedPlaylistProjection(ctx, item, recipe, filepath.Join(manager.cache, key), filepath.Dir(name), options.Cache)
}

func (manager *hlsManager) copiedHLSTimelinePresent(directory string) bool {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Lstat(".copy-timeline")
	return err == nil
}
