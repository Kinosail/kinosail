package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) copiedHLSVariantStart(ctx context.Context, item library.Item, root, policy string, sourceRecipe hlsRecipe, mode string, start float64, startNumber int) (*copiedHLSTimeline, float64, error) {
	timeline, timelineErr := manager.readCopiedHLSTimelineContext(ctx, root, policy)
	if timelineErr != nil && (copiedHLSIndexRequired(ctx) || manager.copiedHLSTimelinePresent(root) || startNumber == 0 && manager.coldCopiedHLSNeedsIndex(ctx, item, sourceRecipe)) {
		return nil, start, errCopiedHLSIndex
	}
	if timeline == nil {
		return nil, start, nil
	}
	if startNumber < 0 || startNumber >= len(timeline.Keys) {
		return nil, start, errCopiedHLSIndex
	}
	switch {
	case copiedHLSAudioFromOrigin(timeline, startNumber, mode):
		return timeline, timeline.point(0), nil
	case copiedHLSWarmAudio(timeline, startNumber, mode):
		return timeline, copiedHLSWarmStart(timeline, startNumber), nil
	default:
		return timeline, timeline.point(startNumber), nil
	}
}

func copiedHLSVariantInputArguments(arguments, input []string, timeline *copiedHLSTimeline, refill *remainingAudioOrigin, recipe hlsRecipe, duration, start float64, startNumber int) []string {
	if startNumber > 0 {
		arguments = append(arguments, "-avoid_negative_ts", "disabled", "-max_delay", "5000000")
	}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	}
	if start > 0 || timeline != nil {
		arguments = append(arguments, copiedHLSVariantSeek(timeline, refill, start)...)
	}
	if copiedHLSVariantInexactSeek(timeline, recipe.mode, startNumber) {
		arguments = append(arguments, "-noaccurate_seek")
	}
	if timeline == nil && refill == nil {
		arguments = hlsSkippedTailInput(arguments, duration, start, recipe)
	}
	return arguments
}

func copiedHLSVariantSeek(timeline *copiedHLSTimeline, refill *remainingAudioOrigin, start float64) []string {
	switch {
	case timeline != nil:
		return []string{"-seek_timestamp", "1", "-ss", copiedHLSInputTime(start)}
	case refill != nil:
		return []string{"-ss", "0.000"}
	default:
		return []string{"-ss", ffmpegSeconds(start)}
	}
}

func copiedHLSVariantInexactSeek(timeline *copiedHLSTimeline, mode string, startNumber int) bool {
	if timeline == nil || mode != "audio-transcode" {
		return false
	}
	if timeline.AudioSource != nil && timeline.AudioSource.Quantum > 0 {
		return true
	}
	return startNumber > 0 && !copiedHLSAudioFromOrigin(timeline, startNumber, mode) && !copiedHLSWarmAudio(timeline, startNumber, mode)
}

func copiedHLSVariantOutputOffset(arguments []string, timeline *copiedHLSTimeline, refill *remainingAudioOrigin, recipe hlsRecipe, startNumber int) []string {
	if timeline != nil || startNumber <= 0 {
		return arguments
	}
	if refill != nil {
		return append(arguments, "-output_ts_offset", refill.output, "-bsf:a", refill.drop)
	}
	if recipe.mode == "remux" || recipe.mode == "audio-transcode" {
		arguments = append(arguments, "-copypriorss", "0")
	}
	return append(arguments, "-output_ts_offset", ffmpegSeconds(recipe.outputTime))
}
