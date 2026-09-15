package server

import (
	"bytes"
	"errors"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	maximumSubtitleCues = 100_000
	subtitleRepeatGap   = 100 * time.Millisecond
)

var (
	subtitleBlocks   = regexp.MustCompile(`\n[ \t]*\n+`)
	subtitleTime     = regexp.MustCompile(`^(?:(\d{1,3}):)?(\d{2}):(\d{2})[,.](\d{3})$`)
	subtitleTags     = regexp.MustCompile(`(?i)</?[^>]{1,128}>`)
	subtitleVoice    = regexp.MustCompile(`(?i)<v(?:\.[^ >]+)*[ \t]+([^>]{1,128})>`)
	subtitleASSStyle = regexp.MustCompile(`\\([ibu])([01])\b`)
	subtitleDrawing  = regexp.MustCompile(`\\p[1-9][0-9]*\b`)
	subtitleASSTags  = regexp.MustCompile(`\{\\[^}]{1,256}\}`)
)

type subtitleCue struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

type cleanedSubtitle struct {
	Data            []byte
	Original        []byte
	Cues            []subtitleCue
	MaxCPS          float64
	Duplicates      int
	Cleanup         []string
	TimingEvidence  string
	Synchronization string
}

func cleanSubtitle(data []byte) (cleanedSubtitle, error) { //nolint:gocognit,cyclop,funlen // One bounded parser owns normalization, validation, cleanup, ordering, and deduplication.
	if len(data) == 0 || len(data) > 4<<20 {
		return cleanedSubtitle{}, errors.New("subtitle text is invalid")
	}
	original := append([]byte(nil), data...)
	data, err := decodeSubtitleText(data)
	if err != nil || len(data) == 0 || len(data) > 4<<20 || bytes.ContainsRune(data, 0) {
		return cleanedSubtitle{}, errors.New("subtitle text is invalid")
	}
	cues, err := parseSubtitleCues(data)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	sort.SliceStable(cues, func(left, right int) bool {
		if cues[left].Start == cues[right].Start {
			return cues[left].End < cues[right].End
		}
		return cues[left].Start < cues[right].Start
	})
	cues = slicesWithoutEmptyCues(cues)
	cues, duplicates := uniqueSubtitleCues(cues)
	if len(cues) == 0 {
		return cleanedSubtitle{}, errors.New("subtitle has no dialogue cues")
	}
	cleaned, err := subtitleDocument(cues, duplicates)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	cleaned.Original = original
	cleaned.Cleanup = []string{"Normalized to UTF-8 SRT"}
	if duplicates > 0 {
		cleaned.Cleanup = append(cleaned.Cleanup, "Removed "+strconv.Itoa(duplicates)+" repeated cues")
	}
	return cleaned, nil
}

func parseSubtitleCues(data []byte) ([]subtitleCue, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[Script Info]") {
		return parseASSSubtitle(data)
	}
	if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<tt") {
		return parseTTMLSubtitle(data)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	blocks := subtitleBlocks.Split(strings.TrimSpace(text), -1)
	if len(blocks) > maximumSubtitleCues+2 {
		return nil, errors.New("subtitle has too many cues")
	}
	cues := make([]subtitleCue, 0, len(blocks))
	for _, block := range blocks {
		cue, found, err := parseSubtitleBlock(block)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		cues = append(cues, cue)
	}
	if len(cues) == 0 || len(cues) > maximumSubtitleCues {
		return nil, errors.New("subtitle has no valid cues")
	}
	return cues, nil
}

func parseSubtitleBlock(block string) (subtitleCue, bool, error) {
	lines := strings.Split(block, "\n")
	timing := subtitleTimingLine(lines)
	if timing < 0 {
		return subtitleCue{}, false, nil
	}
	start, end, ok := parseSubtitleTiming(lines[timing])
	if !ok || end > 36*time.Hour {
		return subtitleCue{}, false, errors.New("subtitle timing is invalid")
	}
	clean := cleanSubtitleCueLines(lines[timing+1:])
	if len(clean) == 0 {
		return subtitleCue{}, false, nil
	}
	return subtitleCue{start, end, balanceSubtitleTags(strings.Join(clean, "\n"))}, true, nil
}

func subtitleTimingLine(lines []string) int {
	for index, line := range lines {
		if strings.Contains(line, "-->") {
			return index
		}
	}
	return -1
}

func cleanSubtitleCueLines(lines []string) []string {
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = cleanSubtitleLine(line); line != "" {
			clean = append(clean, line)
		}
	}
	return clean
}

func cleanSubtitleLine(line string) string {
	line = subtitleVoice.ReplaceAllString(line, "$1: ")
	line = subtitleASSTags.ReplaceAllStringFunc(line, func(block string) string {
		var style strings.Builder
		for _, match := range subtitleASSStyle.FindAllStringSubmatch(block, -1) {
			if match[2] == "1" {
				style.WriteString("<" + match[1] + ">")
			} else {
				style.WriteString("</" + match[1] + ">")
			}
		}
		return style.String()
	})
	line = subtitleTags.ReplaceAllStringFunc(line, func(tag string) string {
		tag = strings.ToLower(tag)
		if oneOf(tag, "<i>", "</i>", "<b>", "</b>", "<u>", "</u>") {
			return tag
		}
		return ""
	})
	line = html.UnescapeString(line)
	line = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) && character != '\t' {
			return -1
		}
		return character
	}, line)
	return norm.NFC.String(strings.Join(strings.Fields(line), " "))
}

func balanceSubtitleTags(text string) string {
	matches := subtitleTags.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	keep, names, stack := make([]bool, len(matches)), make([]string, len(matches)), make([]int, 0, len(matches))
	for index, match := range matches {
		tag := text[match[0]:match[1]]
		name, closing := strings.Trim(tag, "</>"), strings.HasPrefix(tag, "</")
		names[index] = name
		if !closing {
			stack = append(stack, index)
			continue
		}
		if len(stack) == 0 || names[stack[len(stack)-1]] != name {
			continue
		}
		opening := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		keep[opening], keep[index] = true, true
	}
	var output strings.Builder
	previous := 0
	for index, match := range matches {
		output.WriteString(text[previous:match[0]])
		if keep[index] {
			output.WriteString(text[match[0]:match[1]])
		}
		previous = match[1]
	}
	output.WriteString(text[previous:])
	return output.String()
}

func removeSubtitleCredits(text string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		plain := strings.ToLower(subtitleTags.ReplaceAllString(line, ""))
		if strings.HasPrefix(plain, "subtitles by ") || strings.HasPrefix(plain, "subtitle by ") || strings.HasPrefix(plain, "downloaded from ") || strings.HasPrefix(plain, "sync and corrections by ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func slicesWithoutEmptyCues(cues []subtitleCue) []subtitleCue {
	kept := cues[:0]
	for _, cue := range cues {
		if cue.Text != "" {
			kept = append(kept, cue)
		}
	}
	return kept
}

func uniqueSubtitleCues(cues []subtitleCue) ([]subtitleCue, int) {
	seen := make(map[string]struct{}, len(cues))
	kept, duplicates := cues[:0], 0
	for _, cue := range cues {
		key := strconv.FormatInt(int64(cue.Start), 10) + "\x00" + strconv.FormatInt(int64(cue.End), 10) + "\x00" + cue.Text
		if _, found := seen[key]; found {
			duplicates++
			continue
		}
		seen[key] = struct{}{}
		kept = append(kept, cue)
	}
	return kept, duplicates
}

func mergeRepeatedSubtitleCues(cues []subtitleCue) ([]subtitleCue, int) {
	kept, duplicates := cues[:0], 0
	for _, cue := range cues {
		if len(kept) > 0 && cue.Text == kept[len(kept)-1].Text && cue.Start <= kept[len(kept)-1].End+subtitleRepeatGap {
			kept[len(kept)-1].End = max(kept[len(kept)-1].End, cue.End)
			duplicates++
			continue
		}
		kept = append(kept, cue)
	}
	return kept, duplicates
}
