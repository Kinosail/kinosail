package server

import (
	"strings"
	"unicode"
)

const minimumSubtitleScriptLetters = 20

var subtitleLanguageScripts = func() map[string][]*unicode.RangeTable {
	scripts := map[string][]*unicode.RangeTable{}
	for names, tables := range map[string][]*unicode.RangeTable{
		"af ca cs da de en es et eu fi fr gl hr hu id is it lt lv ms nb nl no pl pt ro sk sl sq sv tl tr vi": {unicode.Latin},
		"be bg mk ru uk": {unicode.Cyrillic},
		"el":             {unicode.Greek},
		"he yi":          {unicode.Hebrew},
		"ar fa":          {unicode.Arabic},
		"th":             {unicode.Thai},
		"ko":             {unicode.Hangul, unicode.Han},
		"ja":             {unicode.Hiragana, unicode.Katakana, unicode.Han},
		"zh":             {unicode.Han},
		"ka":             {unicode.Georgian},
		"hy":             {unicode.Armenian},
	} {
		for name := range strings.FieldsSeq(names) {
			scripts[name] = tables
		}
	}
	return scripts
}()

func subtitleScriptMatches(language string, cues []subtitleCue) bool {
	expected := subtitleLanguageScripts[strings.Split(strings.ToLower(canonicalSubtitleLanguage(language)), "-")[0]]
	if expected == nil {
		return true
	}
	letters, matching := 0, 0
	for _, cue := range cues {
		for _, character := range subtitleTags.ReplaceAllString(cue.Text, "") {
			if !unicode.IsLetter(character) {
				continue
			}
			letters++
			if unicode.IsOneOf(expected, character) {
				matching++
			}
		}
	}
	return letters < minimumSubtitleScriptLetters || 2*matching >= letters
}
