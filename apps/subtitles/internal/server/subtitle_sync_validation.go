package server

import (
	"math"
	"time"
)

// Fit continuous drift from separated regional offsets rather than quantizing
// every recording to one of the common broadcast frame-rate conversions.
func continuousSubtitleAlignment(global subtitleAlignment, anchors []subtitleAlignmentAnchor) (subtitleAlignment, bool) {
	if len(anchors) < 3 {
		return global, false
	}
	first, last := anchors[0], anchors[len(anchors)-1]
	if last.At <= first.At {
		return global, false
	}
	slope := float64(last.Offset-first.Offset) / float64(last.At-first.At)
	if math.Abs(slope) > .02 {
		return global, false
	}
	for _, anchor := range anchors[1 : len(anchors)-1] {
		predicted := first.Offset + time.Duration(float64(anchor.At-first.At)*slope)
		if absDuration(anchor.Offset-predicted) > 500*time.Millisecond {
			return global, false
		}
	}
	global.Scale *= 1 + slope
	global.Offset = first.Offset - time.Duration(float64(first.At)*slope)
	return global, true
}

func subtitleAbruptTimingChange(anchors []subtitleAlignmentAnchor) bool {
	for i := 1; i < len(anchors); i++ {
		if absDuration(anchors[i].Offset-anchors[i-1].Offset) > 5*time.Second {
			return true
		}
	}
	return false
}

// These windows do not fit the regional correction. Reject corrections that
// leave a measurable timing error there, including likely alternate cuts.
func validateSubtitleRegions(audio []float64, cues []subtitleCue) bool {
	if len(cues) == 0 {
		return false
	}
	maximum := time.Duration(0)
	for _, cue := range cues {
		maximum = max(maximum, cue.End)
	}
	if maximum > time.Duration(len(audio))*subtitleFrame+2*time.Second {
		return false
	}
	if maximum < 15*time.Minute {
		return true
	}
	frame := subtitleFrame * subtitleCoarseStep
	coarse := averageSubtitleFrames(audio, subtitleCoarseStep)
	validated := 0
	for _, window := range [][2]int{{0, 1}, {2, 4}, {5, 7}, {8, 9}} {
		start, end := maximum*time.Duration(window[0])/9, maximum*time.Duration(window[1])/9
		first, last := max(0, int(start/frame)), min(len(coarse), int(end/frame))
		if last-first < int(time.Minute/frame) {
			continue
		}
		regional := make([]subtitleCue, 0)
		for _, cue := range cues {
			if cue.Start >= start && cue.End <= end {
				regional = append(regional, subtitleCue{Start: cue.Start - start, End: cue.End - start, Text: cue.Text})
			}
		}
		if len(regional) < 5 {
			continue
		}
		activity := subtitleActivity(regional, 1, frame, last-first)
		score, ok := subtitleCorrelation(coarse[first:last], activity, 0)
		if !ok || score < .08 {
			return false
		}
		best, _, shift := bestPiecewiseSubtitleShift(coarse[first:last], activity, 0, int(10*time.Second/frame))
		if best-score > .04 && absDuration(time.Duration(shift)*frame) > time.Second {
			return false
		}
		validated++
	}
	return validated >= 2
}
