package server

import (
	"slices"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type subtitleLanguageSetting string

type subtitleLanguageChoice struct {
	Code, Name string
	Selected   bool
}

var subtitleLanguageCodes = []string{"af", "ar", "bg", "bn", "ca", "cs", "cy", "da", "de", "el", "en", "es", "et", "eu", "fa", "fi", "fil", "fr", "ga", "gl", "he", "hi", "hr", "hu", "hy", "id", "is", "it", "ja", "ka", "kk", "km", "ko", "lt", "lv", "mk", "ml", "mn", "mr", "ms", "nb", "nl", "pa", "pl", "pt", "ro", "ru", "si", "sk", "sl", "sq", "sr", "sv", "sw", "ta", "te", "th", "tr", "uk", "ur", "uz", "vi", "zh", "zu"}

func (setting subtitleLanguageSetting) Choices() []subtitleLanguageChoice {
	codes := subtitleLanguageCodes
	if current := string(setting); current != "" && !slices.Contains(codes, current) {
		codes = append(slices.Clone(codes), current)
	}
	choices := make([]subtitleLanguageChoice, 0, len(codes))
	for _, code := range codes {
		tag, err := language.Parse(code)
		if err != nil {
			continue
		}
		name := display.English.Tags().Name(tag)
		if native := display.Self.Name(tag); native != "" && !strings.EqualFold(native, name) {
			name += " · " + native
		}
		if name == "" {
			name = code
		}
		choices = append(choices, subtitleLanguageChoice{Code: code, Name: name, Selected: code == string(setting)})
	}
	slices.SortFunc(choices, func(left, right subtitleLanguageChoice) int { return strings.Compare(left.Name, right.Name) })
	return choices
}
