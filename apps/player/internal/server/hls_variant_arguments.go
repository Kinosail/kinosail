package server

import (
	"context"
	"github.com/MikeO7/kinosail/packages/library"
)

func hlsVariantInputArguments(ctx context.Context, source string, recipe hlsRecipe, timeline *copiedHLSTimeline, input []string, fromBeginning bool, start float64, startNumber int) ([]string, error) {
	arguments := startupInputArguments(ctx, []string{"-hide_banner", "-loglevel", "error", "-y"})
	if startNumber > 0 {
		arguments = append(arguments, "-avoid_negative_ts", "disabled", "-max_delay", "5000000")
	}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	}
	if start > 0 || timeline != nil {
		seek, err := hlsVariantSeek(timeline, start, startNumber, fromBeginning)
		if err != nil {
			return nil, err
		}
		if timeline != nil {
			arguments = append(arguments, "-seek_timestamp", "1")
		}
		arguments = append(arguments, "-ss", seek)
	}
	return append(arguments, "-i", source), nil
}

func hlsVariantSeek(timeline *copiedHLSTimeline, start float64, startNumber int, fromBeginning bool) (string, error) {
	if timeline == nil {
		if fromBeginning {
			return "0.000", nil
		}
		return ffmpegSeconds(start), nil
	}
	if timeline.AudioOrigin == nil {
		return copiedHLSInputTime(start), nil
	}
	micros, err := copiedAACKeyMicros(timeline, timeline.Keys[startNumber].PTS)
	if err != nil {
		return "", err
	}
	return copiedAACMicrosText(micros), nil
}

func hlsVariantClockArguments(arguments []string, timeline *copiedHLSTimeline, refill *remainingAudioOrigin, outputTime float64, startNumber int) []string {
	if timeline != nil && timeline.AudioOrigin != nil {
		arguments = append(arguments, "-avoid_negative_ts", "disabled")
	}
	if timeline == nil && startNumber > 0 {
		if refill != nil {
			return append(arguments, "-output_ts_offset", refill.output, "-bsf:a", refill.drop)
		}
		return append(arguments, "-output_ts_offset", ffmpegSeconds(outputTime))
	}
	return arguments
}

func (manager *hlsManager) hlsVariantRemainingRefill(ctx context.Context, item library.Item, sourceRecipe, recipe hlsRecipe, start float64, number int, audioRate string, timeline *copiedHLSTimeline) *remainingAudioOrigin {
	if timeline == nil && number > 0 && item.Kind == "audio" {
		return remainingAudioOriginRefill(mediaFactsFor(item, manager.probe.facts(ctx, item)), sourceRecipe, recipe, start, number, audioRate)
	}
	return nil
}
