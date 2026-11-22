package localization

import (
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func LanguageName(tag, code, fallback string) string {
	page, err := language.Parse(tag)
	if err != nil {
		return fallback
	}
	if base, _ := page.Base(); base.String() == "en" {
		return fallback
	}
	subject, err := language.Parse(code)
	if err != nil {
		return fallback
	}
	namer := display.Tags(page)
	if namer == nil {
		return fallback
	}
	name := namer.Name(subject)
	if name == "" {
		return fallback
	}
	if native := display.Self.Name(subject); native != "" && !strings.EqualFold(native, name) {
		name += " · " + native
	}
	return name
}
