package playback

import (
	"math"
	"strconv"
	"strings"
)

func Interlaced(video VideoFacts) bool { return oneOf(video.FieldOrder, "tt", "bb", "tb", "bt") }

func DisplayDimensions(video VideoFacts) (int, int) {
	width, height := video.Width, video.Height
	left, right, ok := strings.Cut(video.SampleAspectRatio, ":")
	numerator, errN := strconv.Atoi(left)
	denominator, errD := strconv.Atoi(right)
	if ok && errN == nil && errD == nil && numerator > 0 && numerator <= 10000 && denominator > 0 && denominator <= 10000 {
		width = int(math.Round(float64(width) * float64(numerator) / float64(denominator)))
	}
	if video.Rotation%180 != 0 {
		width, height = height, width
	}
	return width, height
}

func FitDimensions(width, height, maximumWidth, maximumHeight int) (int, int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	scale := 1.0
	if maximumWidth > 0 {
		scale = math.Min(scale, float64(maximumWidth)/float64(width))
	}
	if maximumHeight > 0 {
		scale = math.Min(scale, float64(maximumHeight)/float64(height))
	}
	return max(2, int(float64(width)*scale)/2*2), max(2, int(float64(height)*scale)/2*2)
}
