package playback

func convertedAudioCodec(facts MediaFacts, index int, client ClientCapabilities) string {
	if facts.Kind == "audio" || facts.Kind == "audiobook" || !Includes(client.AudioCodecs, "eac3") || client.MaxAudioChannels > 0 && client.MaxAudioChannels < 6 {
		return "aac"
	}
	for _, track := range facts.Audio {
		if track.Index == index && track.Channels > 2 {
			return "eac3"
		}
	}
	return "aac"
}

func recipeModeLetter(recipe HLSRecipe) string {
	if recipe.Surround && recipe.Mode == "audio-transcode" {
		return "s"
	}
	return map[string]string{"remux": "r", "audio-transcode": "a", "transcode": "t"}[recipe.Mode]
}
