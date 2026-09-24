package server

func audioEnhancedPlan(plan PlaybackPlan, preferences playbackPreferences, hasAudio bool) (PlaybackPlan, bool) {
	effects := (preferences.DialogueBoost || preferences.NightMode) && hasAudio
	if effects && plan.Allowed && (plan.Mode == "direct" || plan.Mode == "remux") {
		plan.Mode, plan.Reason, plan.AudioCodec = "audio-transcode", "audio-enhancement-requested", "aac"
	}
	return plan, effects
}

func audioEnhancedRecipe(plan PlaybackPlan, preferences playbackPreferences, effects bool) hlsRecipe {
	recipe := recipeFor(plan)
	recipe.dialogueBoost, recipe.normalizeLoudness = effects && preferences.DialogueBoost, effects && preferences.NightMode
	return recipe
}

func applyAudioEnhancementPresentation(result *apiPlayback, plan PlaybackPlan, preferences playbackPreferences, effects bool) {
	if !effects {
		return
	}
	label, description := audioEnhancementPresentation(preferences)
	if plan.Mode == "transcode" {
		result.CompatibleDescription += " " + description
		return
	}
	result.CompatibleLabel, result.CompatibleDescription = label, description
}

func audioEnhancementPresentation(preferences playbackPreferences) (string, string) {
	switch {
	case preferences.DialogueBoost && preferences.NightMode:
		return "Enhancing audio", "Brings dialog forward and keeps loudness consistent."
	case preferences.DialogueBoost:
		return "Boosting dialog", "Brings speech frequencies forward without raising everything else."
	default:
		return "Normalizing loudness", "Keeps quiet and loud moments at a more comfortable level."
	}
}
