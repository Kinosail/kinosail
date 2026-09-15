package server

import "github.com/MikeO7/kinosail/packages/playback"

type hlsRecipe struct {
	mode, burn, codec              string
	width, height, subtitleOrdinal int
	subtitlePath                   string
	outputTime                     float64
	subtitleTime                   float64
	audio, subtitle                int
	toneMap                        bool
	singleQuality                  bool
	maxBitrate                     int64
	omitted                        []PlaybackRange
	offset                         float64
}

func hlsPolicy() playback.HLSRecipePolicy {
	return playback.HLSRecipePolicy{MaxBitrate: maxJellyfinStreamingBitrate, OffsetStepMilliseconds: 100}
}

func sharedHLSRecipe(recipe hlsRecipe) playback.HLSRecipe {
	return playback.HLSRecipe{OutputTime: recipe.outputTime, Width: recipe.width, Height: recipe.height, SubtitleOrdinal: recipe.subtitleOrdinal, SubtitlePath: recipe.subtitlePath, SubtitleTime: recipe.subtitleTime, Mode: recipe.mode, Burn: recipe.burn, Codec: recipe.codec, Audio: recipe.audio, Subtitle: recipe.subtitle, ToneMap: recipe.toneMap, SingleQuality: recipe.singleQuality, MaxBitrate: recipe.maxBitrate, Omitted: recipe.omitted, Offset: recipe.offset}
}

func localHLSRecipe(recipe playback.HLSRecipe) hlsRecipe {
	return hlsRecipe{outputTime: recipe.OutputTime, width: recipe.Width, height: recipe.Height, subtitleOrdinal: recipe.SubtitleOrdinal, subtitlePath: recipe.SubtitlePath, subtitleTime: recipe.SubtitleTime, mode: recipe.Mode, burn: recipe.Burn, codec: recipe.Codec, audio: recipe.Audio, subtitle: recipe.Subtitle, toneMap: recipe.ToneMap, singleQuality: recipe.SingleQuality, maxBitrate: recipe.MaxBitrate, omitted: recipe.Omitted, offset: recipe.Offset}
}

func recipeFor(plan PlaybackPlan) hlsRecipe { return localHLSRecipe(playback.RecipeFor(plan)) }
func (recipe hlsRecipe) token() string      { return sharedHLSRecipe(recipe).Token() }

func timelineFromPlaybackToken(value string) (PlaybackTimeline, error) {
	return playback.TimelineFromPlaybackToken(value, hlsPolicy())
}

func hlsPlanURL(id string, plan PlaybackPlan) string { return playback.HLSPlanURL(id, plan) }

func plannedHLSFile(name string) (hlsRecipe, string, bool) {
	recipe, file, ok := playback.PlannedHLSFile(name, hlsPolicy())
	return localHLSRecipe(recipe), file, ok
}

func hlsRecipeKey(id string, recipe hlsRecipe) string {
	return playback.HLSRecipeKey(id, sharedHLSRecipe(recipe))
}

func validHLSOffset(offset, duration float64) bool { return playback.ValidHLSOffset(offset, duration) }

func hlsWindowRecipe(recipe hlsRecipe, duration float64) (float64, hlsRecipe) {
	start, window := playback.HLSWindowRecipe(sharedHLSRecipe(recipe), duration)
	return start, localHLSRecipe(window)
}

func applyBurnIn(video []string, itemPath string, recipe hlsRecipe) ([]string, []string) {
	return playback.ApplyBurnIn(video, itemPath, sharedHLSRecipe(recipe), hlsPolicy())
}

func automaticSkipAudioArguments(recipe hlsRecipe) []string {
	return playback.AutomaticSkipAudioArguments(sharedHLSRecipe(recipe), hlsPolicy())
}

func ffmpegSeconds(value float64) string { return playback.FFmpegSeconds(value) }
