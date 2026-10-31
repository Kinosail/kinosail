package server

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type (
	AudioFacts    = playback.AudioFacts
	SubtitleFacts = playback.SubtitleFacts
)

func oneOf(value string, allowed ...string) bool {
	return slices.Contains(allowed, value)
}

func subtitleLanguageFromPath(media, subtitle string) string {
	_, language := subtitleTrackLanguage(subtitle, strings.TrimSuffix(media, filepath.Ext(media)), nil)
	if language != "" {
		return language
	}
	return "und"
}

func subtitleRoleFromPath(path string) string {
	if library.SubtitleFileForced(path) {
		return "forced"
	}
	if role := library.SubtitleFileRole(path); role != "" {
		return role
	}
	return "translation"
}
