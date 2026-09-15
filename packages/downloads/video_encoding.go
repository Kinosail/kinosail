package downloads

import (
	"context"
	"errors"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

type downloadVideoEncoding struct {
	options             transcodepolicy.Settings
	width               int
	inputs, maps        []string
	matroska, copyVideo bool
}

func (manager *Manager) videoEncoding(ctx context.Context, item library.Item, quality string, selection *TrackSelection, options transcodepolicy.Settings) (downloadVideoEncoding, error) { //nolint:cyclop // One plan validates the source and selected tracks before reserving encoding resources.
	if options.Encoder == "" {
		return downloadVideoEncoding{}, errors.New("download encoder is unavailable")
	}
	width := map[string]int{"compatible": 1920, "1080p": 1920, "720p": 1280, "480p": 854}[quality]
	var trackInputs, trackMaps []string
	var matroska bool

	if manager.inspect == nil {
		return downloadVideoEncoding{}, errors.New("download media inspection is unavailable")
	}
	facts := manager.inspect(ctx, item)
	var err error
	trackInputs, trackMaps, matroska, err = resolveTracks(facts, item, selection)
	if err != nil {
		return downloadVideoEncoding{}, err
	}
	if facts.Video.Codec == "" {
		return downloadVideoEncoding{}, errors.New("download source could not be inspected")
	}
	copyVideo := quality == "compatible" && compatibleDownloadVideo(facts.Video)
	recipe := playback.HLSRecipe{Mode: "transcode", ToneMap: facts.Video.HDR != "" && facts.Video.HDR != "sdr"}
	if len(facts.Audio) > 0 {
		recipe.Audio = facts.Audio[0].Index
	}
	if _, err := playback.ResolveHLSSource(recipe, facts, item.Subtitles); err != nil {
		return downloadVideoEncoding{}, err
	}
	options = playback.SourceTranscoding(options, facts, recipe)
	sourceWidth, sourceHeight := playback.DisplayDimensions(facts.Video)
	if sourceWidth > 0 && sourceHeight > 0 {
		width, _ = playback.FitDimensions(sourceWidth, sourceHeight, width, map[string]int{"compatible": 1080, "1080p": 1080, "720p": 720, "480p": 480}[quality])
	}
	return downloadVideoEncoding{options, width, trackInputs, trackMaps, matroska, copyVideo}, nil
}

func (plan downloadVideoEncoding) arguments(source, output string) []string {
	args := []string{}
	input, video := transcodepolicy.VideoArguments(plan.options, strconv.Itoa(plan.width))
	if plan.copyVideo {
		input = nil
		video = []string{"-c:v", "copy"}
	}
	args = append(args, input...)
	args = append(args, "-i", source)
	args = append(args, plan.inputs...)
	args = append(args, plan.maps...)
	args = append(args, video...)
	if !plan.copyVideo {
		args = append(args, transcodepolicy.CompatibilityArguments(plan.options.Codec)...)
	}
	args = append(args, "-c:a", "aac", "-b:a", "160k")
	if plan.matroska {
		args = append(args, "-c:s", "copy", "-f", "matroska")
	} else {
		args = append(args, "-c:s", "mov_text", "-movflags", "+faststart")
	}
	args = append(args, "-y", output)
	return args
}

func compatibleDownloadVideo(video playback.VideoFacts) bool {
	return video.Codec == "h264" && video.BitDepth <= 8 && (video.HDR == "" || video.HDR == "sdr") && !playback.Interlaced(video) && compatibleDownloadGeometry(video)
}

func compatibleDownloadGeometry(video playback.VideoFacts) bool {
	return video.Width > 0 && video.Width <= 1920 && video.Height > 0 && video.Height <= 1080 && video.FrameRate <= 60 && video.Rotation == 0 && (video.SampleAspectRatio == "" || video.SampleAspectRatio == "1:1")
}
