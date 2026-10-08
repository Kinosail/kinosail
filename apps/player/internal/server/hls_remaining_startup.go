package server

import (
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// This exception recognizes the observed four-cut AAC clock, not source EOF.
// Inspect the physical manifest before ordinary VOD projection adds absent cuts.
func remainingRoundedAACStartup(manifest []byte, playableDuration float64) ([]string, bool, bool) {
	if invalidHLSSegmentDuration(playableDuration) || playableDuration <= 8.000002 {
		return nil, false, false
	}
	if !playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") {
		return nil, false, false
	}
	segments, durations, valid := remainingAACStartupCuts(manifest)
	if !valid || !remainingAACStartupClock(durations) {
		return nil, false, false
	}
	for index, name := range segments {
		number, _ := hlsSegmentNumber(name)
		if number != index {
			return segments, true, false
		}
	}
	return segments, true, true
}

func remainingAACStartupCuts(manifest []byte) ([]string, []float64, bool) {
	var segments []string
	var durations []float64
	pending := 0.0
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			value, valid := remainingAACStartupDuration(line, pending)
			if !valid {
				return nil, nil, false
			}
			pending = value
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !remainingAACStartupSegment(line, pending, len(segments)) {
			return nil, nil, false
		}
		segments = append(segments, line)
		durations = append(durations, pending)
		pending = 0
	}
	return segments, durations, pending == 0 && len(segments) == 4
}

func remainingAACStartupSegment(line string, pending float64, count int) bool {
	_, valid := hlsSegmentNumber(line)
	return valid && pending > 0 && count < 4
}

func remainingAACStartupDuration(line string, pending float64) (float64, bool) {
	if pending != 0 {
		return 0, false
	}
	value, _, comma := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	duration, err := strconv.ParseFloat(value, 64)
	return duration, comma && err == nil && !invalidHLSSegmentDuration(duration)
}

func remainingAACStartupClock(durations []float64) bool {
	totalBlocks, totalDuration := 0.0, 0.0
	for _, duration := range durations {
		blocks := math.Round(duration * 48_000 / 1024)
		if (blocks != 93 && blocks != 94) || math.Abs(duration-blocks*1024/48_000) > 0.0000005+1e-12 {
			return false
		}
		totalBlocks += blocks
		totalDuration += duration
	}
	return totalBlocks == 375 && totalDuration < 8
}
