package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

func (manager *hlsManager) encodePresentation(ctx context.Context, item library.Item, root string, options transcodeSettings, recipe hlsRecipe, qualities []PlaybackQuality, audioBitrate int64, start float64, startNumber int) error { //nolint:gocognit // One decode feeds the complete switchable presentation.
	device := transcodehardware.DeviceKey(options)
	release, err := manager.acquireHLSEncode(ctx, len(qualities), device, start, startNumber)
	if err != nil {
		return err
	}
	defer release()
	input, _ := videoArguments(options, strconv.Itoa(qualities[len(qualities)-1].Width))
	encoderThreads := startupEncoderThreads(ctx, max(1, runtime.GOMAXPROCS(0)/len(qualities)))
	filterThreads := strconv.Itoa(encoderThreads)
	arguments := append(startupInputArguments(ctx, []string{"-hide_banner", "-loglevel", "error", "-filter_threads", filterThreads, "-filter_complex_threads", filterThreads, "-y"}), input...)
	if startNumber > 0 {
		arguments = append(arguments, "-max_delay", "5000000")
	}
	if start > 0 {
		arguments = append(arguments, "-ss", ffmpegSeconds(start))
	}
	arguments = append(arguments, "-i", item.Path)
	refill, _ := ctx.Value(transcodeHLSOutputKey{}).(*transcodeHLSOutput)
	if refill != nil {
		arguments = refill.input(arguments, item.Path)
	}
	for index, quality := range qualities {
		layoutNumber := startNumber
		if refill != nil {
			layoutNumber = 0
		}
		directory, playlistDirectory, err := preparePresentationDirectories(root, quality.Label, layoutNumber)
		if err != nil {
			return err
		}
		_, video := videoArguments(options, strconv.Itoa(quality.Width))
		mapping, encoded := applyBurnIn(video, item.Path, recipe)
		label := "[v" + strconv.Itoa(index) + "]"
		mapping = presentationVideoMapping(mapping, label)
		arguments = append(arguments, mapping...)
		if audioBitrate > 0 {
			if refill != nil {
				arguments = refill.audio(arguments, recipe.audio, quality.Label)
			} else {
				arguments = append(arguments, "-map", "0:a:"+strconv.Itoa(recipe.audio)+"?")
				arguments = append(arguments, playback.AudioFilterArguments(sharedHLSRecipe(recipe), hlsPolicy())...)
			}
		}
		arguments = append(arguments, "-sn")
		arguments = append(arguments, encoded...)
		arguments = append(arguments, "-threads:v", strconv.Itoa(encoderThreads))
		videoRate := strconv.FormatInt(max(1, (quality.Bitrate-audioBitrate)/1000), 10) + "k"
		arguments = append(arguments, videoCompatibilityArguments(options.Codec)...)
		arguments = append(arguments, "-maxrate", videoRate, "-bufsize", videoRate)
		if audioBitrate > 0 {
			arguments = append(arguments, "-c:a", "aac", "-ac", "2", "-b:a", strconv.FormatInt(audioBitrate/1000, 10)+"k")
		}
		arguments = append(arguments, "-flags", "+cgop", "-force_key_frames", "expr:gte(t,n_forced*2)")
		arguments = append(arguments, "-avoid_negative_ts", "disabled")
		if startNumber > 0 {
			arguments = append(arguments, "-output_ts_offset", ffmpegSeconds(recipe.outputTime))
		}
		if refill != nil {
			arguments = append(arguments, "-t", copiedHLSTime(refill.duration))
		}
		arguments = append(arguments, hlsSegmentArguments("transcode", directory, filepath.Join(playlistDirectory, "index.m3u8"), startNumber)...)
	}
	//nolint:gosec // G204: executable is installation config and input is found only by a Library scan.
	if err := runHLSCommand(ctx, exec.CommandContext(ctx, manager.ffmpeg, arguments...), item.Path, root); err != nil {
		return err
	}
	if refill != nil {
		return finalizePresentationPlaylists(root, qualities, 0)
	}
	return finalizePresentationPlaylists(root, qualities, startNumber)
}

func preparePresentationDirectories(root, label string, startNumber int) (string, string, error) {
	directory := filepath.Join(root, label)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", err
	}
	playlistDirectory := directory
	if startNumber > 0 {
		playlistDirectory = filepath.Join(root, hlsSeekDirectory(startNumber), label)
		if err := os.MkdirAll(playlistDirectory, 0o700); err != nil {
			return "", "", err
		}
	}
	return directory, playlistDirectory, nil
}

func finalizePresentationPlaylists(root string, qualities []PlaybackQuality, startNumber int) error {
	for _, quality := range qualities {
		playlistDirectory := filepath.Join(root, quality.Label)
		if startNumber > 0 {
			playlistDirectory = filepath.Join(root, hlsSeekDirectory(startNumber), quality.Label)
		}
		if err := finalizePlaylist(filepath.Join(playlistDirectory, "index.m3u8")); err != nil {
			return err
		}
	}
	return nil
}

func hlsSegmentArguments(mode, directory, playlist string, startNumber int) []string {
	flags := "temp_file+independent_segments"
	if mode == "remux" || mode == "audio-transcode" {
		flags = "temp_file"
	}
	arguments := []string{"-f", "hls", "-hls_time", "2", "-hls_playlist_type", "event", "-hls_segment_type", "fmp4", "-hls_segment_options", "movflags=+frag_discont+skip_sidx", "-hls_flags", flags, "-hls_fmp4_init_filename", "init.mp4"}
	if startNumber > 0 {
		arguments = append(arguments, "-start_number", strconv.Itoa(startNumber))
	}
	return append(arguments, "-hls_segment_filename", filepath.Join(directory, "segment-%05d.m4s"), playlist)
}

func hlsSeekDirectory(startNumber int) string { return ".seek-" + strconv.Itoa(startNumber) }

func presentationVideoMapping(mapping []string, label string) []string {
	for argument := range mapping {
		mapping[argument] = strings.ReplaceAll(mapping[argument], "[v]", label)
	}
	return mapping
}
