package server

import (
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
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
	container := result.Container
	if container == "" {
		container = lower(item.Container)
	}
	facts := MediaFacts{Kind: item.Kind, FileVersion: sourceVersion(item.Path), Container: normalizeContainer(container), Bitrate: result.Bitrate, Duration: result.Duration, Seekable: true, Video: result.Video, Audio: append([]AudioFacts(nil), result.AudioFacts...), Subtitles: append([]SubtitleFacts(nil), result.SubtitleFacts...), RandomAccess: append([]float64(nil), result.RandomAccess...)}
	for externalIndex, path := range item.Subtitles {
		index := len(facts.Subtitles)
		facts.Subtitles = append(facts.Subtitles, SubtitleFacts{Index: index, SourceIndex: -1, Codec: lower(strings.TrimPrefix(filepath.Ext(path), ".")), Language: subtitleLanguageFromPath(item.Path, path), Role: subtitleRoleFromPath(path), Text: true, External: true, ExternalIndex: externalIndex})
	}
	return facts
}

func normalizeContainer(value string) string  { return playback.NormalizeContainer(value) }
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
	switch {
	case strings.Contains(name, ".sdh."), strings.Contains(name, ".cc."), strings.Contains(name, ".hi."):
		return "captions"
	case strings.Contains(name, ".commentary."):
		return "commentary"
	default:
		return "translation"
	}
}
