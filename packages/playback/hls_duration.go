package playback

func HLSPlaybackDuration(recipe HLSRecipe, duration float64) float64 {
	start, window := HLSWindowRecipe(recipe, duration)
	duration -= start
	for _, omitted := range window.Omitted {
		duration -= omitted.End - omitted.Start
	}
	return max(0, duration)
}
