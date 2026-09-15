package localization

import (
	"html/template"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Message is the stable subset of an i18n source message used by Kinosail.
type Message struct {
	ID, Other string
}

// Catalog applies one product's translated messages through Player's locale rules.
type Catalog struct {
	languages    []Language
	messages     []Message
	translations map[string]map[string]string
}

// NewCatalog compiles one product's bundle for the shared Player locale engine.
func NewCatalog(bundle *i18n.Bundle, languages []Language, messages []Message) *Catalog {
	translations := make(map[string]map[string]string, len(languages))
	for _, supported := range languages {
		localizer := i18n.NewLocalizer(bundle, supported.Tag, "en")
		translated := make(map[string]string, len(messages))
		for _, message := range messages {
			value, err := localizer.Localize(&i18n.LocalizeConfig{
				MessageID:      message.ID,
				DefaultMessage: &i18n.Message{ID: message.ID, Other: message.Other},
			})
			if err == nil {
				translated[message.ID] = value
			}
		}
		translations[supported.Tag] = translated
	}
	return &Catalog{languages: languages, messages: messages, translations: translations}
}

// Direction returns the writing direction for a supported tag.
func (catalog *Catalog) Direction(tag string) string {
	for _, supported := range catalog.languages {
		if supported.Tag == tag {
			return supported.Direction
		}
	}
	return "ltr"
}

// Localize returns a translated message or the supplied source message.
func (catalog *Catalog) Localize(tag, message string) string {
	if translated := catalog.translations[tag][message]; translated != "" {
		return translated
	}
	return message
}

// Knows reports whether the source catalog contains a message.
func (catalog *Catalog) Knows(message string) bool {
	for _, known := range catalog.messages {
		if known.ID == message {
			return true
		}
	}
	return false
}

// PickerHTML renders the full or compact Player language picker.
func (catalog *Catalog) PickerHTML(tag, preference, csrf string, compact bool) template.HTML {
	var picker strings.Builder
	label := template.HTMLEscapeString(catalog.Localize(tag, "Language"))
	picker.WriteString(`<form class="language-picker" action="/language" method="post" aria-label="` + label + `">`)
	if csrf != "" {
		picker.WriteString(`<input type="hidden" name="_csrf" value="` + template.HTMLEscapeString(csrf) + `">`)
	}
	picker.WriteString(`<label>` + label + `<select name="language" aria-label="` + label + `"><option value="auto"`)
	if preference == "auto" {
		picker.WriteString(` selected`)
	}
	picker.WriteString(`>` + template.HTMLEscapeString(catalog.Localize(tag, "Automatic (browser)")) + `</option>`)
	for _, supported := range catalog.languages {
		picker.WriteString(`<option value="` + template.HTMLEscapeString(supported.Tag) + `"`)
		if supported.Tag == preference {
			picker.WriteString(` selected`)
		}
		picker.WriteString(`>` + template.HTMLEscapeString(supported.Name) + `</option>`)
	}
	picker.WriteString(`</select></label><button class="quiet">` + template.HTMLEscapeString(catalog.Localize(tag, "Save")) + `</button>`)
	if compact {
		picker.WriteString(`<a class="mode" href="/language">` + label + `</a>`)
	}
	picker.WriteString(`</form>`)
	return template.HTML(picker.String()) //nolint:gosec // Every dynamic value is escaped above.
}

// TranslateSource translates cataloged text and accessible attributes in HTML templates.
func (catalog *Catalog) TranslateSource(source, tag string) string { //nolint:gocognit // Embedded HTML translation preserves markup while replacing cataloged text nodes.
	var result strings.Builder
	for source != "" {
		if source[0] == '<' {
			end := strings.IndexByte(source, '>')
			if end < 0 {
				result.WriteString(source)
				break
			}
			result.WriteString(catalog.translateTag(source[:end+1], tag))
			source = source[end+1:]
			continue
		}
		if strings.HasPrefix(source, "{{") {
			end := strings.Index(source, "}}")
			if end < 0 {
				result.WriteString(source)
				break
			}
			result.WriteString(source[:end+2])
			source = source[end+2:]
			continue
		}
		end := len(source)
		for _, delimiter := range []string{"<", "{{"} {
			if index := strings.Index(source, delimiter); index >= 0 {
				end = min(end, index)
			}
		}
		result.WriteString(catalog.translateText(source[:end], tag))
		source = source[end:]
	}
	localized := result.String()
	return strings.Replace(localized, `<html lang="en"`, `<html lang="`+tag+`" dir="`+catalog.Direction(tag)+`"`, 1)
}

func (catalog *Catalog) translateText(source, tag string) string {
	for _, message := range catalog.messages {
		if translated := catalog.Localize(tag, message.ID); translated != message.Other {
			source = replaceLocalizedPhrase(source, message.Other, translated)
		}
	}
	return source
}

func (catalog *Catalog) translateTag(source, tag string) string {
	for _, attribute := range []string{`aria-label="`, `placeholder="`, `title="`} {
		for offset := 0; ; {
			start := strings.Index(source[offset:], attribute)
			if start < 0 {
				break
			}
			start += offset + len(attribute)
			end := strings.IndexByte(source[start:], '"')
			if end < 0 {
				break
			}
			end += start
			translated := catalog.translateTemplateText(source[start:end], tag)
			source = source[:start] + translated + source[end:]
			offset = start + len(translated) + 1
		}
	}
	return source
}

func (catalog *Catalog) translateTemplateText(source, tag string) string {
	var result strings.Builder
	for source != "" {
		start := strings.Index(source, "{{")
		if start < 0 {
			result.WriteString(catalog.translateText(source, tag))
			break
		}
		result.WriteString(catalog.translateText(source[:start], tag))
		end := strings.Index(source[start:], "}}")
		if end < 0 {
			result.WriteString(source[start:])
			break
		}
		end += start + 2
		result.WriteString(source[start:end])
		source = source[end:]
	}
	return result.String()
}

func replaceLocalizedPhrase(source, phrase, translated string) string {
	var result strings.Builder
	for phrase != "" {
		index := strings.Index(source, phrase)
		if index < 0 {
			break
		}
		end := index + len(phrase)
		left := index == 0 || !asciiWord(phrase[0]) || !asciiWord(source[index-1])
		right := end == len(source) || !asciiWord(phrase[len(phrase)-1]) || !asciiWord(source[end])
		if left && right {
			result.WriteString(source[:index])
			result.WriteString(translated)
			source = source[end:]
			continue
		}
		result.WriteString(source[:index+1])
		source = source[index+1:]
	}
	result.WriteString(source)
	return result.String()
}

func asciiWord(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
