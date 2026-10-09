package server

import (
	"net/http"
	"net/url"

	"github.com/MikeO7/kinosail/packages/library"
)

func (api apiServices) applyPlaybackSources(request *http.Request, seek playbackSeek, result *apiPlayback, item library.Item, media probeResult, viewer viewerProfile, facts MediaFacts, client ClientCapabilities, plan PlaybackPlan, preferences playbackPreferences, canStream, canTranscode bool) error { //nolint:cyclop // Independent source capabilities remain below the repository complexity ceiling.
	effects := (preferences.DialogueBoost || preferences.NightMode) && len(facts.Audio) > 0
	if canStream && plan.MarkerMode != "server" && !effects {
		result.DirectAllowed = true
		result.Direct = "/media/" + item.ID
		result.DirectType = directMediaType(item.Path, facts)
		result.Subtitles = playbackSubtitleChoices(item, media, api.settings.subtitleLanguage(), api.settings.subtitlesDefault(), api.settings.subtitlePickerLimited())
	}
	if canStream && item.Kind == "video" {
		result.Trickplay = "/trickplay/" + item.ID + "/{second}"
		if plan.MarkerMode == "server" {
			result.Trickplay += "?playbackToken=" + url.QueryEscape(result.ProgressToken)
		}
	}
	if canTranscode {
		if err := api.applyCompatiblePlaybackSource(request, seek, result, item, media, viewer, facts, client, plan, preferences); err != nil {
			return err
		}
	}
	if viewer.Permits("download", viewer.Owner || viewer.Downloads) {
		result.Download = "/download/" + item.ID
	}
	return nil
}

func (api apiServices) applyCompatiblePlaybackSource(request *http.Request, seek playbackSeek, result *apiPlayback, item library.Item, media probeResult, viewer viewerProfile, facts MediaFacts, client ClientCapabilities, plan PlaybackPlan, preferences playbackPreferences) error {
	compatible, effects, err := api.compatibleSeekPlan(request, seek, result.Start, item, media, viewer, facts, client, plan, preferences)
	if err != nil {
		return err
	}
	if compatible.Allowed && compatible.Mode != "direct" {
		recipe := audioEnhancedRecipe(compatible, preferences, effects)
		result.CompatibleDuration = media.Duration
		if compatible.MarkerMode == "server" {
			result.CompatibleDuration = compatible.Timeline.Duration
			result.CompatibleProgressToken = recipe.token()
		}
		result.Compatible = "/hls/" + item.ID + "/p/" + recipe.token() + "/index.m3u8"
		result.CompatiblePlan = &compatible
		result.CompatibleLabel, result.CompatibleDescription = playbackPresentation(compatible)
		applyAudioEnhancementPresentation(result, compatible, preferences, effects)
		result.Qualities = compatible.Qualities
	}
	return nil
}

func compatibleSeekIntent(seek playbackSeek, preferences playbackPreferences) (NetworkIntent, error) {
	intent := NetworkIntent{PreferCompatibility: true}
	if seek.recipe != nil {
		intent.AudioIndex, intent.MaxBitrate = &seek.recipe.audio, seek.recipe.maxBitrate
		if seek.recipe.dialogueBoost != preferences.DialogueBoost || seek.recipe.normalizeLoudness != preferences.NightMode {
			return NetworkIntent{}, errPlaybackSeek
		}
	}
	return intent, nil
}

func (api apiServices) compatibleSeekPlan(request *http.Request, seek playbackSeek, start float64, item library.Item, media probeResult, viewer viewerProfile, facts MediaFacts, client ClientCapabilities, plan PlaybackPlan, preferences playbackPreferences) (PlaybackPlan, bool, error) {
	intent, err := compatibleSeekIntent(seek, preferences)
	if err != nil {
		return PlaybackPlan{}, false, err
	}
	compatible := playbackWithAutomaticSkip(facts, client, viewerPlaybackPolicy(viewer), intent, media.Markers, api.settings.autoSkip())
	if plan.MarkerMode == "server" && plan.Mode != "transcode" {
		compatible = plan
	}
	compatible, effects := audioEnhancedPlan(compatible, preferences, len(facts.Audio) > 0)
	position := savedSeekPosition(start, facts.Duration)
	if seek.position != nil {
		position = *seek.position
	}
	if compatible.MarkerMode != "server" && position > 0 {
		compatible, err = api.hls.exactSeekPlan(request.Context(), item, facts, client, viewerPlaybackPolicy(viewer), compatible, audioEnhancedRecipe(compatible, preferences, effects), position)
		if err != nil {
			return PlaybackPlan{}, false, err
		}
	}
	return compatible, effects, nil
}
