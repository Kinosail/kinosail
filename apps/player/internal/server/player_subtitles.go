package server

import (
	"strings"

	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
	"golang.org/x/text/language"
)

func sidecarSubtitleLanguage(media, subtitle string) string {
	if base, err := language.ParseBase(strings.Split(subtitlelanguage.FileLanguage(media, subtitle), "-")[0]); err == nil {
		return base.String()
	}
	return ""
}

func sameSubtitleLanguage(left, right string) bool {
	leftBase, leftErr := language.ParseBase(left)
	rightBase, rightErr := language.ParseBase(right)
	return leftErr == nil && rightErr == nil && leftBase == rightBase
}
