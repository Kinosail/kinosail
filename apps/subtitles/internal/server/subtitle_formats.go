package server

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

func parseASSSubtitle(data []byte) ([]subtitleCue, error) {
	var cues []subtitleCue
	var format []string
	events := false
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			events = strings.EqualFold(line, "[Events]")
			continue
		}
		if !events {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "format":
			format = strings.Split(strings.ToLower(value), ",")
			seen := map[string]bool{}
			for i := range format {
				format[i] = strings.TrimSpace(format[i])
				if seen[format[i]] {
					return nil, errors.New("duplicate subtitle format field")
				}
				seen[format[i]] = true
			}
			if len(format) > 32 || !seen["start"] || !seen["end"] || format[len(format)-1] != "text" {
				return nil, errors.New("unsupported subtitle format")
			}
		case "dialogue":
			if len(format) == 0 {
				return nil, errors.New("subtitle format is missing")
			}
			fields := strings.SplitN(strings.TrimSpace(value), ",", len(format))
			if len(fields) != len(format) {
				return nil, errors.New("subtitle event is invalid")
			}
			cue := subtitleCue{}
			for i, name := range format {
				if name == "start" || name == "end" {
					value, ok := parseASSTime(strings.TrimSpace(fields[i]))
					if !ok {
						return nil, errors.New("subtitle timing is invalid")
					}
					if name == "start" {
						cue.Start = value
					} else {
						cue.End = value
					}
				}
				if name == "text" {
					// Vector drawing events cannot be represented as dialogue.
					if subtitleDrawing.MatchString(fields[i]) {
						return nil, errors.New("subtitle drawings require the original ASS file")
					}
					text := strings.NewReplacer(`\N`, "\n", `\n`, "\n", `\h`, " ").Replace(fields[i])
					cue.Text = balanceSubtitleTags(strings.Join(cleanSubtitleCueLines(strings.Split(text, "\n")), "\n"))
				}
			}
			if cue.End <= cue.Start || cue.End > 36*time.Hour || cue.Text == "" {
				return nil, errors.New("subtitle event is invalid")
			}
			cues = append(cues, cue)
			if len(cues) > maximumSubtitleCues {
				return nil, errors.New("subtitle has too many cues")
			}
		}
	}
	if len(cues) == 0 {
		return nil, errors.New("subtitle has no valid cues")
	}
	return cues, nil
}

func parseASSTime(value string) (time.Duration, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	return parseSubtitleTime(parts[0] + "." + parts[1] + "0")
}

func parseTTMLSubtitle(data []byte) ([]subtitleCue, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var cues []subtitleCue
	var cue *subtitleCue
	var content strings.Builder
	depth, root := 0, false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("subtitle XML is invalid")
		}
		switch element := token.(type) {
		case xml.Directive:
			return nil, errors.New("subtitle XML directives are unsupported")
		case xml.StartElement:
			depth++
			if depth > 32 {
				return nil, errors.New("subtitle XML is too deep")
			}
			if depth == 1 {
				if root || element.Name.Local != "tt" {
					return nil, errors.New("subtitle XML root is invalid")
				}
				root = true
			}
			attributes := map[string]string{}
			for _, attr := range element.Attr {
				if _, exists := attributes[attr.Name.Local]; exists {
					return nil, errors.New("subtitle XML attributes are ambiguous")
				}
				attributes[attr.Name.Local] = attr.Value
			}
			if element.Name.Local != "p" && (attributes["begin"] != "" || attributes["end"] != "" || attributes["dur"] != "") || attributes["timeBase"] != "" && attributes["timeBase"] != "media" {
				return nil, errors.New("subtitle timing requires conversion with the original format")
			}
			if element.Name.Local == "p" {
				if cue != nil || len(cues) >= maximumSubtitleCues {
					return nil, errors.New("subtitle XML cues are invalid")
				}
				start, startOK := parseTTMLTime(attributes["begin"])
				end, endOK := parseTTMLTime(attributes["end"])
				if attributes["dur"] != "" {
					if attributes["end"] != "" {
						return nil, errors.New("subtitle timing is conflicting")
					}
					var duration time.Duration
					duration, endOK = parseTTMLTime(attributes["dur"])
					end = start + duration
				}
				if !startOK || !endOK || end <= start || end > 36*time.Hour {
					return nil, errors.New("subtitle timing is invalid")
				}
				cue = &subtitleCue{Start: start, End: end}
				content.Reset()
			}
			if element.Name.Local == "br" && cue != nil {
				content.WriteByte('\n')
			}
		case xml.CharData:
			if cue != nil {
				content.Write(element)
			}
		case xml.EndElement:
			if element.Name.Local == "p" && cue != nil {
				cue.Text = strings.Join(cleanSubtitleCueLines(strings.Split(content.String(), "\n")), "\n")
				if cue.Text != "" {
					cues = append(cues, *cue)
				}
				cue = nil
			}
			depth--
		}
	}
	if !root || depth != 0 || len(cues) == 0 {
		return nil, errors.New("subtitle has no valid cues")
	}
	return cues, nil
}

func parseTTMLTime(value string) (time.Duration, bool) {
	if strings.HasSuffix(value, "s") {
		unit := time.Second
		if strings.HasSuffix(value, "ms") {
			unit = time.Millisecond
			value = strings.TrimSuffix(value, "ms")
		} else {
			value = strings.TrimSuffix(value, "s")
		}
		if value == "" || strings.ContainsAny(value, "+-eE ") {
			return 0, false
		}
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || !(number >= 0 && number <= float64(36*time.Hour)/float64(unit)) {
			return 0, false
		}
		return time.Duration(number * float64(unit)), true
	}
	return parseSubtitleTime(value)
}
