package localization

import (
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Message struct {
	ID, Other string
}

type Catalog struct {
	languages []Language
	state     func() *catalogState
	options   func() pickerOptions
}

type catalogState struct {
	messages     []Message
	translations map[string]map[string]string
	groups       map[string]int
	groupTags    []string
}

func NewCatalog(languages []Language, load func() (*i18n.Bundle, []Message)) *Catalog {
	return &Catalog{
		languages: languages,
		state: sync.OnceValue(func() *catalogState {
			bundle, messages := load()
			return newCatalogState(bundle, languages, messages)
		}),
		options: sync.OnceValue(func() pickerOptions { return newPickerOptions(languages) }),
	}
}

func newCatalogState(bundle *i18n.Bundle, languages []Language, messages []Message) *catalogState {
	state := &catalogState{messages: messages, translations: make(map[string]map[string]string, len(languages)), groups: make(map[string]int, len(languages))}
	for _, supported := range languages {
		translated := localizeMessages(bundle, supported.Tag, messages)
		group := slices.IndexFunc(state.groupTags, func(tag string) bool { return maps.Equal(state.translations[tag], translated) })
		if group < 0 {
			group = len(state.groupTags)
			state.groupTags = append(state.groupTags, supported.Tag)
		} else {
			translated = state.translations[state.groupTags[group]]
		}
		state.translations[supported.Tag] = translated
		state.groups[supported.Tag] = group
	}
	return state
}

func localizeMessages(bundle *i18n.Bundle, tag string, messages []Message) map[string]string {
	localizer := i18n.NewLocalizer(bundle, tag, "en")
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
	return translated
}

func (catalog *Catalog) group(tag string) (int, string, bool) {
	state := catalog.state()
	group, ok := state.groups[tag]
	if !ok {
		return 0, "", false
	}
	return group, state.groupTags[group], true
}

func (catalog *Catalog) groupCount() int {
	return len(catalog.state().groupTags)
}

func (catalog *Catalog) Direction(tag string) string {
	for _, supported := range catalog.languages {
		if supported.Tag == tag {
			return supported.Direction
		}
	}
	return "ltr"
}

func (catalog *Catalog) Localize(tag, message string) string {
	if translated := catalog.state().translations[tag][message]; translated != "" {
		return translated
	}
	return message
}

func (catalog *Catalog) Knows(message string) bool {
	for _, known := range catalog.state().messages {
		if known.ID == message {
			return true
		}
	}
	return false
}

func (catalog *Catalog) translateBody(source, tag string) string {
	var result strings.Builder
	result.Grow(len(source))
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
	return result.String()
}

func (catalog *Catalog) translateText(source, tag string) string {
	var translations []string
	for _, message := range catalog.state().messages {
		if message.Other == "" || !strings.Contains(source, message.Other) {
			continue
		}
		if translated := catalog.Localize(tag, message.ID); translated != message.Other {
			marker := translationMarker(len(translations))
			if replaced := replaceLocalizedPhrase(source, message.Other, marker); replaced != source {
				source = replaced
				translations = append(translations, translated)
			}
		}
	}
	for position, translated := range translations {
		source = strings.ReplaceAll(source, translationMarker(position), translated)
	}
	return source
}

func translationMarker(position int) string {
	const digits = "\x01\x02\x03\x04\x05\x06\x07\x08"
	marker := []byte{0}
	for {
		marker = append(marker, digits[position%len(digits)])
		position /= len(digits)
		if position == 0 {
			return string(append(marker, 0))
		}
	}
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
	if phrase == "" || !strings.Contains(source, phrase) {
		return source
	}
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
