package playback

import (
	"errors"
	"math"
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

var ErrHLSSource = errors.New("playback recipe does not match the source")

// ResolveHLSSource validates a URL recipe against scanned source facts before
// creating output. File paths are resolved only from the library's sidecars.
func ResolveHLSSource(recipe HLSRecipe, facts MediaFacts, sidecars []string) (HLSRecipe, error) {
	invalid := ErrHLSSource
	recipe.SubtitlePath, recipe.SubtitleOrdinal, recipe.SubtitleTime, recipe.OutputTime = "", 0, 0, 0
	if !validHLSRecipeBounds(recipe, len(sidecars)) {
		return HLSRecipe{}, invalid
	}
	if !validHLSSourceTimeline(recipe, facts) {
		return HLSRecipe{}, invalid
	}
	if facts.Kind == "audio" || facts.Kind == "audiobook" {
		if !audioOnlyRecipe(recipe) || len(facts.Audio) == 0 {
			return HLSRecipe{}, invalid
		}
		return recipe, nil
	}
	if err := validateHLSVideo(recipe, facts); err != nil {
		return HLSRecipe{}, err
	}
	if recipe.Burn == "" {
		return recipe, nil
	}
	return resolveHLSSubtitle(recipe, facts.Subtitles, sidecars)
}

// SourceTranscoding restricts accelerated decode evidence to its tested input
// and chooses color/CPU filters from this source, never a global HDR switch.
func SourceTranscoding(options transcodepolicy.Settings, facts MediaFacts, recipe HLSRecipe) transcodepolicy.Settings {
	options.ToneMap = recipe.ToneMap
	options.OutputHDR = ""
	options.HardwareToneMap, options.ToneMapInput = "", ""
	options = sourceColor(options, facts.Video.HDR, recipe)
	if options.ToneMap {
		options.ToneMapInput = facts.Video.HDR
		if options.ToneMapInput == "hdr10+" {
			options.ToneMapInput = "hdr10"
		}
		if facts.Video.HDR == "dolby-vision" {
			if facts.Video.DolbyVisionCompatibility == 1 {
				options.ToneMapInput = "hdr10"
			}
			if facts.Video.DolbyVisionCompatibility == 4 {
				options.ToneMapInput = "hlg"
			}
		}
	}
	options.Deinterlace = Interlaced(facts.Video)
	options.SoftwareFilters = recipe.Burn != "" || len(recipe.Omitted) > 0 || facts.Video.Rotation != 0 || facts.Video.SampleAspectRatio != "" && facts.Video.SampleAspectRatio != "1:1"
	options.HardwareDecode = options.HardwareDecode && testedDecodeInput(facts.Video) && options.OutputHDR == ""
	options.HardwareDecode = options.HardwareDecode && !options.ToneMap && !options.Deinterlace && !options.SoftwareFilters
	return options
}

func oneOfInt(value int, allowed ...int) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validHLSRecipeBounds(recipe HLSRecipe, sidecarCount int) bool { //nolint:cyclop // Validate all independent recipe fields before resolving source tracks.
	if !validHLSSelection(recipe) || !finiteTimelineValue(recipe.Offset) || len(recipe.Omitted) > 128 || sidecarCount > 256 {
		return false
	}
	if (recipe.Width != 0 || recipe.Height != 0) && (!validHLSSize(recipe.Width, recipe.Height) || recipe.Mode != "transcode") {
		return false
	}
	if recipe.Codec != "" && (!transcodepolicy.ValidCodec(recipe.Codec) || recipe.Codec == "auto") {
		return false
	}
	if !validHLSRanges(recipe.Omitted) {
		return false
	}
	if !oneOf(recipe.Mode, "remux", "audio-transcode", "transcode") {
		return false
	}
	if (recipe.DialogueBoost || recipe.NormalizeLoudness) && recipe.Mode == "remux" {
		return false
	}

	return true
}

func validHLSRanges(ranges []Range) bool {
	previous := 0.0
	for _, omitted := range ranges {
		if math.IsNaN(omitted.Start) || math.IsInf(omitted.Start, 0) || !finiteTimelineValue(omitted.End) || omitted.Start < previous || omitted.End <= omitted.Start {
			return false
		}
		previous = omitted.End
	}
	return true
}

func validHLSSelection(recipe HLSRecipe) bool {
	return recipe.Audio >= 0 && recipe.Audio <= 31 && recipe.Subtitle >= 0 && recipe.Subtitle <= 255 && oneOf(recipe.Burn, "", "text", "image", "external") && recipe.MaxBitrate >= 0 && recipe.MaxBitrate <= 1000000000
}

func validHLSSize(width, height int) bool {
	return width >= 2 && width <= 1920 && width%2 == 0 && height >= 2 && height <= 1080 && height%2 == 0
}

func audioOnlyRecipe(recipe HLSRecipe) bool {
	return recipe.Mode == "audio-transcode" && recipe.Audio >= 0 && recipe.Audio <= 31 && recipe.Burn == "" && !recipe.ToneMap && recipe.Width == 0 && recipe.Height == 0 && len(recipe.Omitted) == 0 && recipe.Subtitle == 0
}

func validateHLSVideo(recipe HLSRecipe, facts MediaFacts) error {
	if facts.Video.Codec == "" {
		return ErrHLSSource
	}
	if recipe.Mode != "transcode" {
		if !copyableHLSVideo(recipe, facts.Video) {
			return ErrHLSSource
		}
		if !validRemuxAudio(recipe, facts.Audio) {
			return ErrHLSSource
		}
	} else if facts.Video.HDR == "dolby-vision" && !oneOfInt(facts.Video.DolbyVisionCompatibility, 1, 4) {
		return errors.New("this Dolby Vision conversion is unsupported")
	}
	if recipe.ToneMap && (facts.Video.HDR == "" || facts.Video.HDR == "sdr") {
		return ErrHLSSource
	}
	return nil
}

func resolveHLSSubtitle(recipe HLSRecipe, subtitles []SubtitleFacts, sidecars []string) (HLSRecipe, error) {
	ordinal := 0
	for _, subtitle := range subtitles {
		if recipe.Burn == "external" && subtitle.External && subtitle.ExternalIndex == recipe.Subtitle {
			return resolveExternalHLSSubtitle(recipe, subtitle, sidecars)
		}
		if subtitle.External {
			continue
		}
		if subtitle.SourceIndex == recipe.Subtitle && recipe.Burn != "external" {
			if subtitle.Text != (recipe.Burn == "text") {
				return HLSRecipe{}, ErrHLSSource
			}
			recipe.SubtitleOrdinal = ordinal
			return recipe, nil
		}
		ordinal++
	}
	return HLSRecipe{}, ErrHLSSource
}

func resolveExternalHLSSubtitle(recipe HLSRecipe, subtitle SubtitleFacts, sidecars []string) (HLSRecipe, error) {
	if !subtitle.Text || recipe.Subtitle < 0 || recipe.Subtitle >= len(sidecars) {
		return HLSRecipe{}, ErrHLSSource
	}
	if sidecars[recipe.Subtitle] == "" || len(sidecars[recipe.Subtitle]) > 4096 || strings.ContainsAny(sidecars[recipe.Subtitle], "\x00\r\n") {
		return HLSRecipe{}, ErrHLSSource
	}
	recipe.SubtitlePath, recipe.SubtitleOrdinal = sidecars[recipe.Subtitle], 0
	return recipe, nil
}

func sourceColor(options transcodepolicy.Settings, hdr string, recipe HLSRecipe) transcodepolicy.Settings {
	if recipe.Mode == "transcode" && hdr != "" && hdr != "sdr" {
		if !recipe.ToneMap && options.Codec == "hevc" && oneOf(hdr, "hdr10", "hlg") {
			options.OutputHDR = hdr
		} else {
			options.ToneMap = true
		}
	}
	return options
}

func testedDecodeInput(video VideoFacts) bool {
	return video.Codec == "h264" && video.BitDepth <= 8 && video.Rotation == 0 && (video.SampleAspectRatio == "" || video.SampleAspectRatio == "1:1")
}

func validHLSSourceTimeline(recipe HLSRecipe, facts MediaFacts) bool {
	audioFound := len(facts.Audio) == 0 && recipe.Audio == 0
	for _, audio := range facts.Audio {
		if audio.Index == recipe.Audio {
			audioFound = true
		}
	}
	if !audioFound || recipe.Offset < 0 || recipe.Offset > 0 && !ValidHLSOffset(recipe.Offset, facts.Duration) {
		return false
	}
	for _, omitted := range recipe.Omitted {
		if omitted.End > facts.Duration {
			return false
		}
	}
	return true
}

func validRemuxAudio(recipe HLSRecipe, audioTracks []AudioFacts) bool {
	if recipe.Mode == "remux" {
		for _, audio := range audioTracks {
			if audio.Index == recipe.Audio && !oneOf(audio.Codec, "aac", "mp3", "opus", "ac3", "eac3", "flac") {
				return false
			}
		}
	}
	return true
}

func copyableHLSVideo(recipe HLSRecipe, video VideoFacts) bool {
	return recipe.Burn == "" && !recipe.ToneMap && oneOf(video.Codec, "h264", "hevc", "av1", "vp9")
}
