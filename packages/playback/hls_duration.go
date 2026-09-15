package playback

// HLSPlaybackDuration applies Player's seek and automatic-skip timeline to source duration.
func HLSPlaybackDuration(recipe HLSRecipe, duration float64) float64 {
	start, window := HLSWindowRecipe(recipe, duration)
	duration -= start
	for _, omitted := range window.Omitted {
		duration -= omitted.End - omitted.Start
	}
	return max(0, duration)
}
