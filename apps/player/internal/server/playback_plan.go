package server

import (
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"golang.org/x/text/language"
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
	label := lower(subtitleLabel(media, subtitle))
	if language, _, _ := strings.Cut(label, " · "); len(language) >= 2 && len(language) <= 8 {
		return language
	}
	return "und"
}

func subtitleRoleFromPath(path string) string {
	name := lower(filepath.Base(path))
	parts := strings.Split(strings.TrimSuffix(name, filepath.Ext(name)), ".")
	if len(parts) >= 3 && parts[len(parts)-1] == "hi" {
		if tag, err := language.Parse(parts[len(parts)-2]); err == nil && tag != language.Und {
			return "captions"
		}
	}
	switch {
	case strings.Contains(name, ".sdh."), strings.Contains(name, ".cc."):
		return "captions"
	case strings.Contains(name, ".commentary."):
		return "commentary"
	default:
		return ""
	}
}
