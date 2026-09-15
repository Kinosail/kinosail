package server

import (
	"context"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) encodeAudioVariant(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe, window hlsRecipe, duration, start float64, startNumber int) error {
	bitrate := int64(192_000)
	if recipe.maxBitrate > 0 {
		bitrate = min(bitrate, recipe.maxBitrate)
	}
	quality := PlaybackQuality{Label: "audio", Bitrate: max(1, bitrate)}
	results := make(chan error, 1)
	go func() {
		results <- manager.encodeVariant(ctx, item, directory, "audio", "0", "", strconv.FormatInt(bitrate, 10), duration, options, recipe, window, start, startNumber)
	}()
	if startNumber > 0 {
		return <-results
	}
	return publishVariants(ctx, item.Path, directory, options.Cache, "", []PlaybackQuality{quality}, results, 1, false)
}
