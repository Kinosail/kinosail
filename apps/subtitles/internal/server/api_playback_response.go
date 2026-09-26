package server

import (
	"net/url"

	"github.com/MikeO7/kinosail/packages/library"
)

func (api apiServices) applyPlaybackSources(result *apiPlayback, item library.Item, media probeResult, viewer viewerProfile, facts MediaFacts, client ClientCapabilities, plan PlaybackPlan, canStream, canTranscode bool) { //nolint:cyclop // Independent source capabilities remain below the repository complexity ceiling.
	if canStream && plan.MarkerMode != "server" {
		result.Direct = "/media/" + item.ID
		result.DirectType = directMediaType(item.Path, facts)
		result.Subtitles = playbackSubtitles(item, media, api.subtitles, api.settings.subtitleLanguages(), api.settings.subtitlePreference(), api.settings.subtitlePickerLimited(), api.settings.subtitlesDefault())
	}
	if canStream && item.Kind == "video" {
		result.Trickplay = "/trickplay/" + item.ID + "/{second}"
		if plan.MarkerMode == "server" {
			result.Trickplay += "?playbackToken=" + url.QueryEscape(result.ProgressToken)
		}
	}
	if canTranscode {
		compatible := playbackWithAutomaticSkip(facts, client, viewerPlaybackPolicy(viewer), NetworkIntent{PreferCompatibility: true}, media.Markers, api.settings.autoSkip())
		if plan.MarkerMode == "server" && plan.Mode != "transcode" {
			compatible = plan
		}
		result.Compatible = hlsPlanURL(item.ID, compatible)
		result.CompatiblePlan = &compatible
		result.CompatibleLabel, result.CompatibleDescription = playbackPresentation(compatible)
		result.Qualities = compatible.Qualities
	}
	if viewer.Permits("download", viewer.Owner || viewer.Downloads) {
		result.Download = "/download/" + item.ID
	}
}
