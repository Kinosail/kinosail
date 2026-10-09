package server

import (
	"context"
	"os/exec"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) encodeVariant(ctx context.Context, item library.Item, root, name, width, videoRate, audioRate string, duration float64, options transcodeSettings, sourceRecipe, recipe hlsRecipe, start float64, startNumber int) error { //nolint:cyclop,funlen // One FFmpeg command is assembled from the validated playback recipe.
	release, err := manager.acquireHLSEncode(ctx, 1, "", start, startNumber)
	if err != nil {
		return err
	}
	defer release()
	ctx, directory, playlist, output, err := manager.prepareCopiedHLSOutput(ctx, root, name, options.Cache, recipe.mode, startNumber)
	if err != nil {
		return err
	}
	defer output.close()
	timeline, mapErr := manager.readCopiedHLSTimeline(root, options.Cache)
	if mapErr != nil && (copiedAACPolicyRequired(options.Cache) || manager.copiedHLSTimelinePresent(root)) {
		return errCopiedHLSIndex
	}
	if timeline != nil {
		if startNumber < 0 || startNumber >= len(timeline.Keys) {
			return errCopiedHLSIndex
		}
		start = timeline.point(startNumber)
	}
	var refill *remainingAudioOrigin
	if timeline == nil && startNumber > 0 && item.Kind == "audio" {
		refill = remainingAudioOriginRefill(mediaFactsFor(item, manager.probe.facts(ctx, item)), sourceRecipe, recipe, start, startNumber, audioRate)
	}
	input, video := videoArguments(options, width)
	arguments := startupInputArguments(ctx, []string{"-hide_banner", "-loglevel", "error", "-y"})
	if startNumber > 0 {
		arguments = append(arguments, "-avoid_negative_ts", "disabled", "-max_delay", "5000000")
	}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	}
	if start > 0 || timeline != nil {
		seek := ffmpegSeconds(start)
		if timeline != nil {
			seek = copiedHLSInputTime(start)
			if timeline.AudioOrigin != nil {
				micros, err := copiedAACKeyMicros(timeline, timeline.Keys[startNumber].PTS)
				if err != nil {
					return err
				}
				seek = copiedAACMicrosText(micros)
			}
			arguments = append(arguments, "-seek_timestamp", "1")
		} else if refill != nil {
			seek = "0.000"
		}
		arguments = append(arguments, "-ss", seek)
	}
	arguments = append(arguments, "-i", item.Path)
	copyInput := "0"
	if recipe.mode != "transcode" && len(sourceRecipe.omitted) > 0 {
		arguments, copyInput, err = hlsSkipInput(arguments, directory, item.Path, duration, sourceRecipe)
		if err != nil {
			return err
		}
	}
	arguments, err = playback.HLSCodecArguments(playback.HLSCodecInput{AudioOnly: item.Kind == "audio" || item.Kind == "audiobook", Arguments: arguments, Video: video, Compatibility: videoCompatibilityArguments(options.Codec), ItemPath: item.Path, VideoRate: videoRate, AudioRate: audioRate, CopyInput: copyInput, Recipe: sharedHLSRecipe(recipe), Policy: hlsPolicy()})
	if err != nil {
		return err
	}
	if timeline != nil && timeline.AudioOrigin != nil {
		arguments = append(arguments, "-avoid_negative_ts", "disabled")
	}
	if timeline == nil && startNumber > 0 {
		if refill != nil {
			arguments = append(arguments, "-output_ts_offset", refill.output, "-bsf:a", refill.drop)
		} else {
			arguments = append(arguments, "-output_ts_offset", ffmpegSeconds(recipe.outputTime))
		}
	}
	arguments, err = copiedHLSProducerArguments(arguments, timeline, startNumber)
	if err != nil {
		return err
	}
	arguments = append(arguments, output.arguments()...)
	arguments = append(arguments, copiedAACSegmentArguments(hlsSegmentArguments(recipe.mode, directory, playlist, startNumber), timeline, startNumber)...)
	//nolint:gosec // G204: executable is installation config and input is found only by a Library scan.
	command := exec.CommandContext(ctx, manager.ffmpeg, arguments...)
	if err := output.run(command, item.Path, root); err != nil {
		return err
	}
	return output.publish(manager, ctx, item, recipe, options.Cache)
}
