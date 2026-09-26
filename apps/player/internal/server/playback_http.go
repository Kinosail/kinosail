package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
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

func subtitleTrackLabel(code, role string, forced bool) string {
	label := "Subtitles"
	switch {
	case forced:
		label = "Forced"
	case role == "captions":
		label = "Captions"
	case role == "commentary":
		label = "Commentary"
	}
	if tag, err := language.Parse(code); err == nil && tag != language.Und {
		if name := display.English.Tags().Name(tag); name != "" {
			return name + " · " + label
		}
	}
	return label
}

func playbackSubtitles(item library.Item, media probeResult, preferredLanguage string, enabled bool) []subtitleTrack {
	tracks := make([]subtitleTrack, 0, len(media.SubtitleFacts)+len(item.Subtitles))
	for _, track := range media.SubtitleFacts {
		if !track.Text {
			continue
		}
		role := track.Role
		if role == "translation" {
			role = ""
		}
		kind := "subtitles"
		if role == "captions" {
			kind = "captions"
		}
		tracks = append(tracks, subtitleTrack{Label: subtitleTrackLabel(track.Language, role, track.Forced), Source: fmt.Sprintf("/subtitle/%s/embedded/%d", item.ID, track.SourceIndex), Default: track.Default, Language: track.Language, Role: role, Kind: kind, Forced: track.Forced, Embedded: true})
	}
	for index, path := range item.Subtitles {
		role := subtitleRoleFromPath(path)
		forced := strings.Contains(strings.ToLower(filepath.Base(path)), ".forced.")
		kind := "subtitles"
		if role == "captions" {
			kind = "captions"
		}
		language := sidecarSubtitleLanguage(item.Path, path)
		tracks = append(tracks, subtitleTrack{Label: subtitleTrackLabel(language, role, forced), Source: fmt.Sprintf("/subtitle/%s/%d", item.ID, index), Default: index == 0, Language: language, Role: role, Kind: kind, Forced: forced})
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
