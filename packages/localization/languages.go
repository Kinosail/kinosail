package localization

import (
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Language describes one locale that a Kinosail product can expose.
type Language struct {
	Tag, Name, Direction string
}

var supportedLanguageTags = []string{
	"en", "es", "de", "fr", "pt-BR", "zh-Hans", "it", "nl", "pl", "ru", "ja", "ko", "ar", "tr", "uk", "pt-PT", "zh-Hant", "sv",
	"ab", "af", "as", "az", "be-BY", "bg-BG", "bn", "bn-BD", "br", "bs", "ca", "ch", "ckb", "cs", "cy", "da", "dv", "el", "en-GB", "en-US", "eo", "es-AR", "es-MX", "es-419", "es-DO", "et", "eu", "fa", "fi", "fil", "fo", "fr-CA", "ga", "gl", "gsw", "gu", "he", "he-IL", "hi-IN", "hr", "ht", "hu", "hy", "id", "is-IS", "jbo", "ka", "kab", "kk", "kn", "kw", "ky", "lb", "lt-LT", "lv", "mg", "mi", "mk", "ml", "mn", "mr", "ms", "mt", "my", "nb", "nds", "ne", "nn", "oc", "pa", "pl", "pt", "ro", "si", "sk", "sl-SI", "so", "sq", "sr", "sw", "ta", "te", "th", "ug", "ur-PK", "uz", "vi", "zh-CN", "zh-HK", "zh-TW", "zu",
}

var nativeLanguageNames = map[string]string{
	"en": "English", "es": "Español", "de": "Deutsch", "fr": "Français", "pt-BR": "Português (Brasil)", "zh-Hans": "简体中文", "it": "Italiano", "nl": "Nederlands", "pl": "Polski", "ru": "Русский", "ja": "日本語", "ko": "한국어", "ar": "العربية", "tr": "Türkçe", "uk": "Українська", "pt-PT": "Português (Portugal)", "zh-Hant": "繁體中文", "sv": "Svenska",
}

var rtlLanguageBases = map[string]struct{}{
	"ar": {}, "ckb": {}, "dv": {}, "fa": {}, "he": {}, "ug": {}, "ur": {},
}

var translatedLanguageTags = map[string]struct{}{
	"en": {}, "es": {}, "de": {}, "fr": {}, "pt-BR": {}, "zh-Hans": {}, "it": {}, "nl": {}, "pl": {}, "ru": {}, "ja": {}, "ko": {}, "ar": {}, "tr": {}, "uk": {}, "pt-PT": {}, "zh-Hant": {}, "sv": {},
}

// SupportedLanguages returns the Player locale catalog in display order.
func SupportedLanguages() []Language {
	result := make([]Language, 0, len(supportedLanguageTags))
	seen := make(map[string]struct{}, len(supportedLanguageTags))
	for _, tag := range supportedLanguageTags {
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		parsed := language.MustParse(tag)
		name := nativeLanguageNames[tag]
		if name == "" {
			name = display.Self.Name(parsed)
		}
		if name == "" {
			name = display.English.Tags().Name(parsed)
		}
		base, _ := parsed.Base()
		direction := "ltr"
		if _, ok := rtlLanguageBases[strings.ToLower(base.String())]; ok {
			direction = "rtl"
		}
		result = append(result, Language{Tag: tag, Name: name, Direction: direction})
	}
	return result
}

// ExactSupportedLanguage returns the canonical spelling of an exact supported tag.
func ExactSupportedLanguage(languages []Language, value string) (string, bool) {
	parsed, err := language.Parse(value)
	if err != nil {
		return "", false
	}
	for _, supported := range languages {
		candidate, _ := language.Parse(supported.Tag)
		if candidate == parsed {
			return supported.Tag, true
		}
	}
	return "", false
}

// IsTranslated reports whether Kinosail maintains a translated catalog for the tag.
func IsTranslated(tag string) bool {
	_, ok := translatedLanguageTags[tag]
	return ok
}
