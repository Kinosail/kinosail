package server

import (
	"github.com/MikeO7/kinosail/packages/playback"
)

var (
	automaticSkipTimeline    = playback.TimelineForAutomaticSkip
	automaticSkipSelection   = playback.AutomaticSkipTypes
	automaticStartOffset     = playback.AutomaticStart
	remainingPlaybackMarkers = playback.RemainingMarkers
	timelineChapters         = playback.TimelineChapters
)

func playbackWithAutomaticSkip(facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, intent NetworkIntent, markers []playbackMarker, enabled []string) PlaybackPlan {
	return playback.DecideWithAutomaticSkip(facts, client, policy, intent, markers, enabled, playback.DecisionPolicy{})
}
