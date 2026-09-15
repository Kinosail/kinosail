package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

func (manager *hlsManager) encodeVariant(ctx context.Context, item library.Item, root, name, width, videoRate, audioRate string, duration float64, options transcodeSettings, sourceRecipe, recipe hlsRecipe, start float64) error { //nolint:cyclop,funlen // One FFmpeg command is assembled from the validated playback recipe.
	release, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return err
	}
	defer release()
	directory := filepath.Join(root, name)
	//nolint:gosec // G703: root is the validated cache path and name is a fixed rendition.
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	playlist := filepath.Join(directory, "index.m3u8")
	input, video := videoArguments(options, width)
	arguments := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if recipe.mode == "transcode" {
		arguments = append(arguments, input...)
	}
	if start > 0 {
		arguments = append(arguments, "-ss", ffmpegSeconds(start))
	}
	arguments = append(arguments, "-i", item.Path)
	copyInput := "0"
	if recipe.mode != "transcode" && len(sourceRecipe.omitted) > 0 {
		concat, err := hlsmanifest.WriteSkipConcat(directory, item.Path, duration, sourceRecipe.omitted)
		if err != nil {
			return err
		}
		if sourceRecipe.offset > 0 {
			arguments = append(arguments, "-ss", ffmpegSeconds(sourceRecipe.offset))
		}
		arguments, copyInput = append(arguments, "-f", "concat", "-safe", "0", "-i", concat), "1"
	}
	arguments, err = playback.HLSCodecArguments(playback.HLSCodecInput{Arguments: arguments, Video: video, Compatibility: videoCompatibilityArguments(options.Codec), ItemPath: item.Path, VideoRate: videoRate, AudioRate: audioRate, CopyInput: copyInput, Recipe: sharedHLSRecipe(recipe), Policy: hlsPolicy()})
	if err != nil {
		return err
	}
	arguments = append(arguments, hlsSegmentArguments(recipe.mode, directory, playlist)...)
	//nolint:gosec // G204: executable is installation config and input is found only by a Library scan.
	command := exec.CommandContext(ctx, manager.ffmpeg, arguments...)
	if err := runHLSCommand(command, item.Path, root); err != nil {
		return err
	}
	return finalizePlaylist(playlist)
}
