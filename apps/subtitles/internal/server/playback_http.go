package server

import (
	"fmt"
	"net/http"
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

func requestedVideoCodecs(request *http.Request) ([]string, error) {
	return playback.RequestedVideoCodecs(request)
}

func subtitleRoleLabel(role string) string { return playback.SubtitleRoleLabel(role) }

func playbackSubtitles(item library.Item, media probeResult, provider *subtitleProvider, language string, enabled bool) []subtitleTrack {
	tracks := make([]subtitleTrack, 0, len(media.SubtitleFacts)+len(item.Subtitles)+1)
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
		tracks = append(tracks, subtitleTrack{Label: subtitleLabel(item.Path, path), Source: fmt.Sprintf("/subtitle/%s/%d", item.ID, index), Default: index == 0})
	}
	if provider.cached(item.ID, language) != "" {
		tracks = append(tracks, subtitleTrack{Label: strings.ToUpper(language) + " · Provider", Source: "/subtitles/" + item.ID + "/" + language, Default: len(tracks) == 0})
	}
	selectDefaultTextSubtitle(tracks, enabled)
	return tracks
}

func selectDefaultTextSubtitle(tracks []subtitleTrack, enabled bool) {
	selected := -1
	if enabled && len(tracks) > 0 {
		selected = 0
		for index := range tracks {
			if tracks[index].Default {
				selected = index
				break
			}
		}
	}
	for index := range tracks {
		tracks[index].Default = index == selected
	}
}

func requestedPlaybackCapabilities(request *http.Request, settings *settingsStore) (ClientCapabilities, error) {
	hdrFormats, audioChannels, err := playback.RequestedDisplayCapabilities(request)
	if err != nil {
		return ClientCapabilities{}, err
	}
	videoCodecs, err := requestedVideoCodecs(request)
	if err != nil {
		return ClientCapabilities{}, err
	}
	audioCodecs, err := playback.RequestedAudioCodecs(request)
	if err != nil {
		return ClientCapabilities{}, err
	}
	client := browserPlaybackCapabilities(settings, videoCodecs)
	if audioCodecs != nil {
		client.AudioCodecs = audioCodecs
	}
	if hdrFormats != nil {
		client.HDRFormats = hdrFormats
	}
	if audioChannels > 0 {
		client.MaxAudioChannels = audioChannels
	}
	return client, nil
}
