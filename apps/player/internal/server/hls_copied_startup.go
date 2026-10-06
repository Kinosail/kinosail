package server

import (
	"context"
	"math"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) reusableCopiedHLS(ctx context.Context, directory, source, policy string, recipe hlsRecipe) bool {
	if !seekCacheFresh(directory, source, policy) {
		return false
	}
	if recipe.mode != "remux" {
		return true
	}
	timeline, err := readCopiedHLSTimeline(directory, policy)
	return err == nil && timeline.Clock != nil && ctx.Err() == nil
}

func (manager *hlsManager) bindCopiedHLSTimeline(ctx context.Context, directory string, startNumber int) error {
	preparation, ok := ctx.Value(startupEncodingKey{}).(*startupEncoding)
	if !ok || preparation.timeline == nil || startNumber != 0 {
		return nil
	}
	return writeCopiedHLSTimeline(directory, preparation.timeline)
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
	}
	return arguments, nil
}

func (manager *hlsManager) copiedPlaylistProjection(ctx context.Context, item library.Item, recipe hlsRecipe, directory, rendition, policy string) func([]byte) []byte {
	return func(manifest []byte) []byte {
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		timeline, err := readCopiedHLSTimeline(directory, policy)
		if err == nil {
			if result, valid := copiedHLSManifest(manifest, timeline); valid {
				return result
			}
		}
		// Unknown future cuts remain a growing EVENT; EOF correction reads actual
		// generated media, never the source container's format duration.
		if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
			manifest = manager.completedCopiedHLSProjection(ctx, filepath.Join(directory, rendition), manifest)
		}
		if manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return nil
		}
		return manifest
	}
}

func (manager *hlsManager) copiedStartupProjection(item library.Item, recipe hlsRecipe, directory string) func([]byte) []byte {
	if recipe.mode != "remux" {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	timeline, err := readCopiedHLSTimeline(directory, options.Cache)
	return func(manifest []byte) []byte {
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
	if recipe.mode != "remux" || filepath.Dir(name) == "." || filepath.Ext(name) != ".m3u8" {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return func([]byte) []byte { return nil }
	}
	return manager.copiedPlaylistProjection(ctx, item, recipe, filepath.Join(manager.cache, key), filepath.Dir(name), options.Cache)
}
