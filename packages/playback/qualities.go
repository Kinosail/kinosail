package playback

import (
	"math"
	"strconv"
)

func AdaptiveQualities(maxWidth, maxHeight int, frameRate float64, maxBitrate int64) []PlaybackQuality { //nolint:cyclop,gocognit // Source fitting and pruning share one ordered ladder pass.
	specs := []PlaybackQuality{{"360p", 640, 360, 493_000, frameRate}, {"432p", 768, 432, 1_228_000, frameRate}, {"540p", 960, 540, 2_128_000, frameRate}, {"720p", 1280, 720, 3_128_000, frameRate}, {"1080p", 1920, 1080, 6_128_000, frameRate}}
	if maxWidth <= 0 || maxHeight <= 0 {
		maxWidth, maxHeight = 1920, 1080
	}
	qualities := make([]PlaybackQuality, 0, len(specs))
	for _, spec := range specs {
		scale := math.Min(1, math.Sqrt(float64(spec.Width*spec.Height)/float64(maxWidth*maxHeight)))
		quality := spec
		quality.Width = evenDimension(float64(maxWidth) * scale)
		quality.Height = evenDimension(float64(maxHeight) * float64(quality.Width) / float64(maxWidth))
		if scale == 1 {
			quality.Label = QualityLabel(quality.Width, quality.Height)
		}
		if maxBitrate == 0 || quality.Bitrate <= maxBitrate {
			qualities = append(qualities, quality)
		}
		if scale == 1 {
			break
		}
	}
	if len(qualities) == 0 && maxBitrate > 0 {
		quality := specs[0]
		scale := math.Min(1, math.Sqrt(float64(quality.Width*quality.Height)/float64(maxWidth*maxHeight)))
		quality.Width, quality.Bitrate = evenDimension(float64(maxWidth)*scale), maxBitrate
		quality.Height = evenDimension(float64(maxHeight) * float64(quality.Width) / float64(maxWidth))
		qualities = append(qualities, quality)
	}
	return qualities
}

func QualityLabel(width, height int) string {
	for _, tier := range [][2]int{{3840, 2160}, {1920, 1080}, {1280, 720}, {960, 540}, {768, 432}, {640, 360}} {
		if width >= tier[0] && height <= tier[1] {
			return strconv.Itoa(tier[1]) + "p"
		}
	}
	return strconv.Itoa(height) + "p"
}

func evenDimension(value float64) int { return max(2, int(math.Round(value/2))*2) }
