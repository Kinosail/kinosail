package server

import (
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
)

type (
	MediaFacts         = playback.MediaFacts
	VideoFacts         = playback.VideoFacts
	AudioFacts         = playback.AudioFacts
	SubtitleFacts      = playback.SubtitleFacts
	ClientCapabilities = playback.ClientCapabilities
	ViewerPolicy       = playback.ViewerPolicy
	NetworkIntent      = playback.NetworkIntent
	PlaybackPlan       = playback.PlaybackPlan
	PlaybackQuality    = playback.PlaybackQuality
	PlaybackRange      = playback.Range
	PlaybackTimeline   = playback.Timeline
)

func lower(value string) string            { return playback.Lower(value) }
func minimumPositiveInt(values ...int) int { return playback.MinimumPositiveInt(values...) }

func mediaFactsFor(item library.Item, result probeResult) MediaFacts {
	return result.MediaFacts(item, sourceVersion(item.Path), subtitleLanguageFromPath, subtitleRoleFromPath)
}

func browserCapabilities() ClientCapabilities { return playback.BrowserCapabilities() }

func viewerPlaybackPolicy(viewer viewerProfile) ViewerPolicy {
	return ViewerPolicy{AllowPlayback: viewer.Permits("stream", true), AllowTranscode: viewer.Permits("stream", viewer.Owner || viewer.Transcode)}
}

func subtitleLanguageFromPath(media, subtitle string) string {
	if language := subtitlelanguage.FileLanguage(media, subtitle); language != "" {
		return language
	}
	return "und"
}

func subtitleRoleFromPath(path string) string { return library.SubtitleFileRole(path) }
