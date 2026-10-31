package configuration

import (
	"strings"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
)

func validSubtitleLanguage(language string) bool {
	_, ok := subtitlelanguage.NormalizeTag(language)
	return ok
}

func validSubtitleProviderCredential(raw string) bool {
	return strings.TrimSpace(raw) == raw && federation.BoundedIdentityText(raw, 4096)
}
