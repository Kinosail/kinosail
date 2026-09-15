package server

import (
	"path/filepath"
	"strings"

	"golang.org/x/text/language"
)

func sidecarSubtitleLanguage(media, subtitle string) string {
	mediaName := strings.TrimSuffix(filepath.Base(media), filepath.Ext(media))
	subtitleName := strings.TrimSuffix(filepath.Base(subtitle), filepath.Ext(subtitle))
	suffix := strings.TrimPrefix(subtitleName, mediaName+".")
	code, _, _ := strings.Cut(suffix, ".")
	if base, err := language.ParseBase(strings.ToLower(code)); err == nil {
		return base.String()
	}
	return ""
}

func sameSubtitleLanguage(left, right string) bool {
	leftBase, leftErr := language.ParseBase(left)
	rightBase, rightErr := language.ParseBase(right)
	return leftErr == nil && rightErr == nil && leftBase == rightBase
}
