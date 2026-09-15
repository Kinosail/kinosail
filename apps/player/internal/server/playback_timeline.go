package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/playback"
)

var (
	automaticSkipTimeline    = playback.TimelineForAutomaticSkip
	automaticSkipSelection   = playback.AutomaticSkipTypes
	automaticStartOffset     = playback.AutomaticStart
	remainingPlaybackMarkers = playback.RemainingMarkers
	timelineChapters         = playback.TimelineChapters
)

func (api *jellyfinAPI) jellyfinMarkerTimeline(request *http.Request, media probeResult) []playbackMarker {
	viewer := currentViewer(request)
	return playback.JellyfinMarkers(media.Duration, media.Markers, api.settings.autoSkip(), viewer.Owner || viewer.Transcode)
}

func playbackWithAutomaticSkip(facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, intent NetworkIntent, markers []playbackMarker, enabled []string) PlaybackPlan {
	return playback.DecideWithAutomaticSkip(facts, client, policy, intent, markers, enabled, playback.DecisionPolicy{PreferCompatibleAudio: true})
}
