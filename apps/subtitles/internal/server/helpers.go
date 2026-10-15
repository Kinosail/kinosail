package server

import (
	"path/filepath"
	"slices"
	"strings"

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
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, ".forced."):
		return "forced"
	case strings.Contains(name, ".sdh."), strings.Contains(name, ".cc."), strings.Contains(name, ".hi."):
		return "captions"
	case strings.Contains(name, ".commentary."):
		return "commentary"
	default:
		return "translation"
	}
}
