package server

import (
	"math"
	"time"
)

func findSubtitleAlignment(audio []float64, cues []subtitleCue) (subtitleAlignment, bool) { //nolint:gocognit,cyclop // Coarse search, refinement, and confidence margin are one bounded alignment operation.
	if len(audio) < int(time.Minute/subtitleFrame) || len(cues) < 5 {
		return subtitleAlignment{}, false
	}
	group := max(subtitleCoarseStep, (len(audio)+32767)/32768)
	coarseAudio := averageSubtitleFrames(audio, group)
	coarseFrame := subtitleFrame * time.Duration(group)
	maximumShift := int(subtitleSyncRange / coarseFrame)
	best, runner := subtitleAlignment{Score: -1}, -1.0
	for _, scale := range subtitleTimeScales {
		activity := subtitleActivity(cues, scale, coarseFrame, len(coarseAudio))
		for shift := -maximumShift; shift <= maximumShift; shift++ {
			score, ok := subtitleCorrelation(coarseAudio, activity, shift)
			if !ok {
				continue
			}
			if score > best.Score {
				if best.Score >= 0 && (scale != best.Scale || subtitleAbsInt(shift-int(best.Offset/coarseFrame)) > 8) {
					runner = max(runner, best.Score)
				}
				best = subtitleAlignment{Scale: scale, Offset: time.Duration(shift) * coarseFrame, Score: score}
			} else if scale != best.Scale || subtitleAbsInt(shift-int(best.Offset/coarseFrame)) > 8 {
				runner = max(runner, score)
			}
		}
	}
	best = refineSubtitleAlignment(audio, cues, best)
	best.Margin = best.Score - runner
	return best, best.Score >= 0.12 && best.Margin >= 0.015
}

func refineSubtitleAlignment(audio []float64, cues []subtitleCue, coarse subtitleAlignment) subtitleAlignment {
	activity := subtitleActivity(cues, coarse.Scale, subtitleFrame, len(audio))
	center := int(coarse.Offset / subtitleFrame)
	best := coarse
	for shift := center - max(subtitleCoarseStep, (len(audio)+32767)/32768); shift <= center+max(subtitleCoarseStep, (len(audio)+32767)/32768); shift++ {
		if score, ok := subtitleCorrelation(audio, activity, shift); ok && score > best.Score {
			best.Score = score
			best.Offset = time.Duration(shift) * subtitleFrame
		}
	}
	return best
}

func subtitleActivity(cues []subtitleCue, scale float64, frame time.Duration, length int) []float64 {
	activity := make([]float64, length)
	for _, cue := range cues {
		start := max(0, int(time.Duration(float64(cue.Start)*scale)/frame))
		end := min(length, int(time.Duration(float64(cue.End)*scale)/frame)+1)
		for index := start; index < end; index++ {
			activity[index] = 1
		}
	}
	return activity
}

func subtitleCorrelation(audio, activity []float64, shift int) (float64, bool) {
	start, end := max(0, shift), min(len(audio), len(activity)+shift)
	if end-start < min(len(audio), len(activity))*4/5 {
		return 0, false
	}
	count := float64(end - start)
	var sumAudio, sumActivity, sumSquaresAudio, sumSquaresActivity, sumProduct float64
	for audioIndex := start; audioIndex < end; audioIndex++ {
		subtitle := activity[audioIndex-shift]
		voice := audio[audioIndex]
		sumAudio += voice
		sumActivity += subtitle
		sumSquaresAudio += voice * voice
		sumSquaresActivity += subtitle * subtitle
		sumProduct += voice * subtitle
	}
	denominator := math.Sqrt((count*sumSquaresAudio - sumAudio*sumAudio) * (count*sumSquaresActivity - sumActivity*sumActivity))
	if denominator == 0 || sumActivity < 30 || count-sumActivity < 30 {
		return 0, false
	}
	return (count*sumProduct - sumAudio*sumActivity) / denominator, true
}

func averageSubtitleFrames(values []float64, group int) []float64 {
	averages := make([]float64, 0, (len(values)+group-1)/group)
	for start := 0; start < len(values); start += group {
		end, sum := min(start+group, len(values)), 0.0
		for _, value := range values[start:end] {
			sum += value
		}
		averages = append(averages, sum/float64(end-start))
	}
	return averages
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func subtitleAbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
