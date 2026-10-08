package server

import (
	"math"
	"strconv"
)

type remainingAudioOrigin struct {
	output string
	drop   string
}

// A short origin replay retains the AAC encoder history of the existing prefix.
// Keep the replay bounded to the qualified sample grid and plain FLAC mapping.
func remainingAudioOriginRefill(facts MediaFacts, source, window hlsRecipe, start float64, number int, rate string) *remainingAudioOrigin {
	if !remainingOriginSource(facts, start) || rate != "192000" || !(start > 0) || number <= 0 ||
		!remainingOriginRecipes(source, window, start) {
		return nil
	}
	target, valid := remainingOriginTarget(start, number, facts.Duration)
	if !valid {
		return nil
	}
	samples := int64(target * 48000)
	if float64(samples) != target*48000 || samples%1024 != 0 {
		return nil
	}
	cut := samples - 1024
	return &remainingAudioOrigin{
		output: strconv.FormatFloat(target-float64(cut)/48000, 'f', -1, 64),
		drop:   "noise=amount=0:drop=lt(pts\\," + strconv.FormatInt(cut, 10) + ")",
	}
}

func remainingOriginSource(facts MediaFacts, start float64) bool {
	if facts.Kind != "audio" || facts.Container != "flac" || facts.Video.Codec != "" ||
		len(facts.Audio) != 1 || !(facts.Duration > start) || math.IsInf(facts.Duration, 0) {
		return false
	}
	return remainingOriginTrack(facts.Audio[0])
}

func remainingOriginTrack(track AudioFacts) bool {
	return track.Index == 0 && track.SourceIndex == 0 && track.Codec == "flac" &&
		track.SampleRate == 48000 && track.Channels == 2 && track.ChannelLayout == "stereo"
}

func remainingOriginRecipes(source, window hlsRecipe, start float64) bool {
	return source.offset == start && source.outputTime == start && window.offset == 0 && window.outputTime == start &&
		remainingPlainAudio(source) && remainingPlainAudio(window)
}

func remainingOriginTarget(start float64, number int, duration float64) (float64, bool) {
	// EXTINF is rounded to six decimals. Admit only its accumulated rounding
	// around the existing three-decimal FFmpeg target, never another codec cut.
	target, err := strconv.ParseFloat(ffmpegSeconds(start), 64)
	return target, err == nil && target > 0 && target <= 8 && duration > target && target == float64(number)*2 &&
		math.Abs(start-target) <= float64(number)*0.0000005+1e-12
}

func remainingPlainAudio(recipe hlsRecipe) bool {
	return recipe.mode == "audio-transcode" && recipe.audio == 0 && len(recipe.omitted) == 0 &&
		!recipe.dialogueBoost && !recipe.normalizeLoudness
}
