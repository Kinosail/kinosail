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
			var err error
			format, err = assFormat(value)
			if err != nil {
				return nil, err
			}
		case "dialogue":
			cue, err := assDialogue(value, format)
			if err != nil {
				return nil, err
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

type ttmlParser struct {
	cues    []subtitleCue
	cue     *subtitleCue
	content strings.Builder
	depth   int
	root    bool
}

func parseTTMLSubtitle(data []byte) ([]subtitleCue, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var parser ttmlParser
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("subtitle XML is invalid")
		}
		if err = parser.consume(token); err != nil {
			return nil, err
		}
	}
	if !parser.root || parser.depth != 0 || len(parser.cues) == 0 {
		return nil, errors.New("subtitle has no valid cues")
	}
	return parser.cues, nil
}

func (p *ttmlParser) consume(token xml.Token) error {
	switch element := token.(type) {
	case xml.Directive:
		return errors.New("subtitle XML directives are unsupported")
	case xml.StartElement:
		return p.start(element)
	case xml.CharData:
		if p.cue != nil {
			p.content.Write(element)
		}
	case xml.EndElement:
		if element.Name.Local == "p" && p.cue != nil {
			p.cue.Text = strings.Join(cleanSubtitleCueLines(strings.Split(p.content.String(), "\n")), "\n")
			if p.cue.Text != "" {
				p.cues = append(p.cues, *p.cue)
			}
			p.cue = nil
		}
		p.depth--
	}
	return nil
}

func (p *ttmlParser) start(element xml.StartElement) error {
	p.depth++
	if p.depth > 32 {
		return errors.New("subtitle XML is too deep")
	}
	if err := p.checkRoot(element.Name.Local); err != nil {
		return err
	}
	attributes, err := ttmlAttributes(element)
	if err != nil {
		return err
	}
	if element.Name.Local == "p" {
		if p.cue != nil || len(p.cues) >= maximumSubtitleCues {
			return errors.New("subtitle XML cues are invalid")
		}
		p.cue, err = ttmlCue(attributes)
		if err != nil {
			return err
		}
		p.content.Reset()
	}
	if element.Name.Local == "br" && p.cue != nil {
		p.content.WriteByte('\n')
	}
	return nil
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

func assFormat(value string) ([]string, error) {
	format := strings.Split(strings.ToLower(value), ",")
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
	return format, nil
}

func assDialogue(value string, format []string) (subtitleCue, error) {
	if len(format) == 0 {
		return subtitleCue{}, errors.New("subtitle format is missing")
	}
	fields := strings.SplitN(strings.TrimSpace(value), ",", len(format))
	if len(fields) != len(format) {
		return subtitleCue{}, errors.New("subtitle event is invalid")
	}
	cue := subtitleCue{}
	for i, name := range format {
		switch name {
		case "start", "end":
			if err := assTiming(&cue, name, fields[i]); err != nil {
				return subtitleCue{}, err
			}
		case "text":
			// Vector drawing events cannot be represented as dialogue.
			if subtitleDrawing.MatchString(fields[i]) {
				return subtitleCue{}, errors.New("subtitle drawings require the original ASS file")
			}
			text := strings.NewReplacer(`\N`, "\n", `\n`, "\n", `\h`, " ").Replace(fields[i])
			cue.Text = balanceSubtitleTags(strings.Join(cleanSubtitleCueLines(strings.Split(text, "\n")), "\n"))
		}
	}
	if !validASSCue(cue) {
		return subtitleCue{}, errors.New("subtitle event is invalid")
	}
	return cue, nil
}

func ttmlAttributes(element xml.StartElement) (map[string]string, error) {
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
	return attributes, nil
}

func ttmlCue(attributes map[string]string) (*subtitleCue, error) {
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
	return &subtitleCue{Start: start, End: end}, nil
}

func (p *ttmlParser) checkRoot(name string) error {
	if p.depth == 1 {
		if p.root || name != "tt" {
			return errors.New("subtitle XML root is invalid")
		}
		p.root = true
	}
	return nil
}

func assTiming(cue *subtitleCue, name, raw string) error {
	value, ok := parseASSTime(strings.TrimSpace(raw))
	if !ok {
		return errors.New("subtitle timing is invalid")
	}
	if name == "start" {
		cue.Start = value
	} else {
		cue.End = value
	}
	return nil
}

func validASSCue(cue subtitleCue) bool {
	return cue.End > cue.Start && cue.End <= 36*time.Hour && cue.Text != ""
}
