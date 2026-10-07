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
	if facts.Kind != "audio" || facts.Container != "flac" || facts.Video.Codec != "" ||
		len(facts.Audio) != 1 || !(facts.Duration > start) || math.IsInf(facts.Duration, 0) ||
		rate != "192000" || !(start > 0 && start <= 8) || number <= 0 || start != float64(number)*2 ||
		source.offset != start || source.outputTime != start || window.offset != 0 || window.outputTime != start ||
		!remainingPlainAudio(source) || !remainingPlainAudio(window) {
		return nil
	}
	track := facts.Audio[0]
	if track.Index != 0 || track.SourceIndex != 0 || track.Codec != "flac" || track.SampleRate != 48000 || track.Channels != 2 || track.ChannelLayout != "stereo" {
		return nil
	}
	samples := int64(start * 48000)
	if float64(samples) != start*48000 || samples%1024 != 0 {
		return nil
	}
	cut := samples - 1024
	return &remainingAudioOrigin{
		output: strconv.FormatFloat(start-float64(cut)/48000, 'f', -1, 64),
		drop:   "noise=amount=0:drop=lt(pts\\," + strconv.FormatInt(cut, 10) + ")",
	}
}

func remainingPlainAudio(recipe hlsRecipe) bool {
	return recipe.mode == "audio-transcode" && recipe.audio == 0 && len(recipe.omitted) == 0 &&
		!recipe.dialogueBoost && !recipe.normalizeLoudness
}
