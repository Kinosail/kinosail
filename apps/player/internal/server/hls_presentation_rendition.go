package server

import (
	"path/filepath"
	"strconv"

	"github.com/MikeO7/kinosail/packages/playback"
)

type presentationEncoding struct {
	options        transcodeSettings
	recipe         hlsRecipe
	refill         *transcodeHLSOutput
	path           string
	root           string
	graph          string
	audioGraph     string
	audioBitrate   int64
	encoderThreads int
	startNumber    int
}

func (encoding presentationEncoding) renditionArguments(arguments []string, index int, quality PlaybackQuality) ([]string, error) {
	layoutNumber := encoding.startNumber
	if encoding.refill != nil {
		layoutNumber = 0
	}
	directory, playlistDirectory, err := preparePresentationDirectories(encoding.root, quality.Label, layoutNumber)
	if err != nil {
		return nil, err
	}
	_, video := videoArguments(encoding.options, strconv.Itoa(quality.Width))
	mapping, encoded := applyBurnIn(video, encoding.path, encoding.recipe)
	label := "[v" + strconv.Itoa(index) + "]"
	mapping = presentationVideoMapping(mapping, label)
	if encoding.graph != "" {
		mapping, encoded = []string{"-map", label}, video[2:]
	}
	arguments = append(arguments, mapping...)
	arguments = encoding.audioMapping(arguments, index, quality.Label)
	arguments = append(arguments, "-sn")
	arguments = append(arguments, encoded...)
	arguments = append(arguments, "-threads:v", strconv.Itoa(encoding.encoderThreads))
	videoRate := strconv.FormatInt(max(1, (quality.Bitrate-encoding.audioBitrate)/1000), 10) + "k"
	arguments = append(arguments, videoCompatibilityArguments(encoding.options.Codec)...)
	arguments = append(arguments, "-maxrate", videoRate, "-bufsize", videoRate)
	if encoding.audioBitrate > 0 {
		arguments = append(arguments, "-c:a", "aac", "-ac", "2", "-b:a", strconv.FormatInt(encoding.audioBitrate/1000, 10)+"k")
	}
	arguments = append(arguments, "-flags", "+cgop", "-force_key_frames", "expr:gte(t,n_forced*2)")
	arguments = append(arguments, "-avoid_negative_ts", "disabled")
	if encoding.startNumber > 0 {
		arguments = append(arguments, "-output_ts_offset", ffmpegSeconds(encoding.recipe.outputTime))
	}
	if encoding.refill != nil {
		arguments = append(arguments, "-t", copiedHLSTime(encoding.refill.duration))
	}
	return append(arguments, hlsSegmentArguments("transcode", directory, filepath.Join(playlistDirectory, "index.m3u8"), encoding.startNumber)...), nil
}

func (encoding presentationEncoding) audioMapping(arguments []string, index int, label string) []string {
	if encoding.audioBitrate <= 0 {
		return arguments
	}
	switch {
	case encoding.refill != nil:
		return encoding.refill.audio(arguments, encoding.recipe.audio, label)
	case encoding.audioGraph != "":
		return append(arguments, "-map", "[a"+strconv.Itoa(index)+"]")
	default:
		arguments = append(arguments, "-map", "0:a:"+strconv.Itoa(encoding.recipe.audio)+"?")
		return append(arguments, playback.AudioFilterArguments(sharedHLSRecipe(encoding.recipe), hlsPolicy())...)
	}
}
