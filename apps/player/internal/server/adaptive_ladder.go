package server

import (
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

func adaptiveQualities(maxWidth, maxHeight int, frameRate float64, maxBitrate int64) []PlaybackQuality {
	return playback.AdaptiveQualities(maxWidth, maxHeight, frameRate, maxBitrate)
}

func qualityLabel(width, height int) string { return playback.QualityLabel(width, height) }

func ladderRungs(qualities []PlaybackQuality) []workload.Rung {
	rungs := make([]workload.Rung, len(qualities))
	for index, quality := range qualities {
		rungs[index] = workload.Rung{Width: quality.Width, Height: quality.Height, FrameRate: quality.FrameRate}
	}
	return rungs
}
