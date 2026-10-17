package playback

import (
	"strings"
	"unicode"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type SubtitleOption struct {
	Language, Role, Title       string
	Forced, Converted, External bool
	Default                     bool
}

var redundantSubtitleTitles = []string{"subtitles", "subtitle", "subs", "full", "default", "complete", "normal", "regular", "text", "srt", "ass", "pgs", "sdh", "cc", "hi", "forced", "captions", "closed captions"}

func SubtitleKind(option SubtitleOption) string {
	switch {
	case option.Forced:
		return "forced"
	case option.Role == "captions":
		return "captions"
	case option.Role == "commentary":
		return "commentary"
	default:
		return "regular"
	}
}

func SubtitleLanguageName(code string) string {
	tag, err := language.Parse(strings.TrimSpace(code))
	if err != nil || tag == language.Und {
		return ""
	}
	if name := display.English.Tags().Name(tag); name != "" {
		return name
	}
	base, _ := tag.Base()
	return display.English.Languages().Name(base)
}

func SubtitleLabel(option SubtitleOption) string {
	name := SubtitleLanguageName(option.Language)
	title := subtitleTitleHint(option.Title, name)
	if name == "" {
		name = "Subtitles"
	}
	parts := []string{name}
	switch SubtitleKind(option) {
	case "captions":
		parts[0] = name + " [CC]"
	case "forced":
		parts = append(parts, "Foreign dialogue only")
	case "commentary":
		if !strings.Contains(strings.ToLower(title), "commentary") {
			parts = append(parts, "Commentary")
		}
	}
	if title != "" {
		parts = append(parts, title)
	}
	if option.Converted {
		parts = append(parts, "Converted from image")
	}
	return strings.Join(parts, " · ")
}

func SubtitleDetail(option SubtitleOption) string {
	details := map[string]string{
		"captions":   "Includes sound effects and speaker names for viewers who are deaf or hard of hearing.",
		"forced":     "Shows only signs and lines in another language.",
		"commentary": "Text of a commentary track.",
	}
	detail := details[SubtitleKind(option)]
	if option.Converted {
		detail = strings.TrimSpace(detail + " Read from image subtitles by text recognition, so some words can be wrong.")
	}
	return detail
}

func subtitleTitleHint(title, languageName string) string {
	title = strings.Join(strings.FieldsFunc(title, func(character rune) bool { return unicode.IsSpace(character) || unicode.IsControl(character) }), " ")
	if title == "" || len([]rune(title)) > 40 || strings.ContainsAny(title, "<>{}\\") {
		return ""
	}
	normalized := strings.ToLower(strings.Trim(title, "()[] -_.,"))
	for _, word := range []string{strings.ToLower(languageName), "english"} {
		if word != "" {
			normalized = strings.TrimSpace(strings.Trim(strings.ReplaceAll(normalized, word, ""), "()[] -_.,"))
		}
	}
	for _, redundant := range redundantSubtitleTitles {
		normalized = strings.TrimSpace(strings.Trim(strings.ReplaceAll(" "+normalized+" ", " "+redundant+" ", " "), "()[] -_.,"))
	}
	if normalized == "" {
		return ""
	}
	return title
}
