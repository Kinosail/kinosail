package server

import "github.com/MikeO7/kinosail/packages/playback"

func adaptiveQualities(maxWidth, maxHeight int, frameRate float64, maxBitrate int64) []PlaybackQuality {
	return playback.AdaptiveQualities(maxWidth, maxHeight, frameRate, maxBitrate)
}

func qualityLabel(width, height int) string { return playback.QualityLabel(width, height) }
