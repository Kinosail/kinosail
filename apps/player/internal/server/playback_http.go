package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func playbackModeIntent(settings *settingsStore) NetworkIntent {
	mode := settings.playbackMode()
	return NetworkIntent{ForceDirect: mode == "direct", PreferCompatibility: mode == "compatible", PreferDirect: mode == "automatic"}
}

func browserPlaybackCapabilities(settings *settingsStore, reported []string) ClientCapabilities {
	capabilities := browserCapabilities()
	if reported != nil {
		capabilities.VideoCodecs = reported
	}
	capabilities.TranscodeVideoCodecs = []string{settings.preferredVideoCodec(reported)}
	return capabilities
}

func browserPlaybackCapabilitiesForRequest(request *http.Request, settings *settingsStore, reported []string) ClientCapabilities {
	capabilities := browserPlaybackCapabilities(settings, reported)
	userAgent := request.UserAgent()
	if strings.Contains(userAgent, "AppleWebKit/") && (strings.Contains(userAgent, "Mobile/") || strings.Contains(userAgent, "Safari/") && !strings.Contains(userAgent, "Chrome/")) {
		capabilities.AudioCodecs = append(capabilities.AudioCodecs, "ac3", "eac3")
	}
	return capabilities
}

func requestedVideoCodecs(request *http.Request) ([]string, error) {
	return playback.RequestedVideoCodecs(request)
}

func subtitleRoleLabel(role string) string { return playback.SubtitleRoleLabel(role) }

func playbackSubtitles(item library.Item, media probeResult, preferredLanguage string, enabled bool) []subtitleTrack {
	tracks := make([]subtitleTrack, 0, len(media.SubtitleFacts)+len(item.Subtitles))
	for _, track := range media.SubtitleFacts {
		if !track.Text {
			continue
		}
		kind := "subtitles"
		if track.Role == "captions" {
			kind = "captions"
		}
		tracks = append(tracks, subtitleTrack{Label: strings.ToUpper(track.Language) + " · " + subtitleRoleLabel(track.Role), Source: fmt.Sprintf("/subtitle/%s/embedded/%d", item.ID, track.SourceIndex), Default: track.Default, Language: track.Language, Role: track.Role, Kind: kind, Forced: track.Forced, Embedded: true})
	}
	for index, path := range item.Subtitles {
		tracks = append(tracks, subtitleTrack{Label: subtitleLabel(item.Path, path), Source: fmt.Sprintf("/subtitle/%s/%d", item.ID, index), Default: index == 0, Language: sidecarSubtitleLanguage(item.Path, path), Forced: strings.Contains(strings.ToLower(filepath.Base(path)), ".forced.")})
	}
	selectDefaultTextSubtitle(tracks, preferredLanguage, enabled)
	return tracks
}

func selectDefaultTextSubtitle(tracks []subtitleTrack, preferredLanguage string, enabled bool) {
	selected := defaultTextSubtitle(tracks, preferredLanguage, enabled)
	for index := range tracks {
		tracks[index].Default = index == selected
	}
}

func defaultTextSubtitle(tracks []subtitleTrack, preferredLanguage string, enabled bool) int {
	if !enabled || len(tracks) == 0 {
		return -1
	}
	if index := slices.IndexFunc(tracks, func(track subtitleTrack) bool {
		return !track.Forced && sameSubtitleLanguage(track.Language, preferredLanguage)
	}); index >= 0 {
		return index
	}
	if index := slices.IndexFunc(tracks, func(track subtitleTrack) bool { return !track.Forced && track.Default }); index >= 0 {
		return index
	}
	return slices.IndexFunc(tracks, func(track subtitleTrack) bool { return !track.Forced })
}
