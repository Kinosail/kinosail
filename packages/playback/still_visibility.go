package playback

import (
	"image"
	"image/jpeg"
	"os"
)

const (
	visibleStillMean  = 18
	visibleStillRange = 24
	stillSamples      = 24
)

func visibleStill(path string) bool {
	file, err := os.Open(path) // #nosec G304 -- callers pass a cache path they generated.
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	frame, err := jpeg.Decode(file)
	if err != nil {
		return false
	}
	bounds := frame.Bounds()
	if bounds.Empty() {
		return false
	}
	total, lowest, highest := 0, 255, 0
	for row := range stillSamples {
		for column := range stillSamples {
			x := bounds.Min.X + (2*column+1)*bounds.Dx()/(2*stillSamples)
			y := bounds.Min.Y + (2*row+1)*bounds.Dy()/(2*stillSamples)
			value := luma(frame, x, y)
			total += value
			lowest, highest = min(lowest, value), max(highest, value)
		}
	}
	return total/(stillSamples*stillSamples) >= visibleStillMean && highest-lowest >= visibleStillRange
}

func luma(frame image.Image, x, y int) int {
	red, green, blue, _ := frame.At(x, y).RGBA()
	return int((299*red + 587*green + 114*blue) / 1000 >> 8)
}
