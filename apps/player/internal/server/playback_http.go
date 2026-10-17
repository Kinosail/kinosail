package server

import (
	"net/http"
	"strings"

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
