package server

import (
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func subtitleSidecarPath(item library.Item, language string) string {
	return strings.TrimSuffix(item.Path, filepath.Ext(item.Path)) + "." + language + ".srt"
}

func validLanguage(language string) bool {
	_, ok := subtitlelanguage.NormalizeTag(language)
	return ok
}
