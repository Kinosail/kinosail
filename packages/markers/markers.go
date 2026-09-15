package markers

import (
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/metadata"
)

var markerTypes = []string{"intro", "recap", "commercial", "outro", "credits"}

// Marker describes one skippable playback range.
type Marker struct {
	Type   string  `json:"type,omitempty"`
	Label  string  `json:"label"`
	Start  float64 `json:"start,omitempty"`
	End    float64 `json:"end"`
	Source string  `json:"source,omitempty"`
}

// DetectPlaybackMarkers classifies named chapters as playback markers.
func DetectPlaybackMarkers(chapters []metadata.Chapter) []Marker { //nolint:cyclop // All supported chapter-name classes are explicit here.
	markers := make([]Marker, 0)
	if len(chapters) > 4096 {
		return markers
	}
	for _, chapter := range chapters {
		if len(chapter.Title) > 512 || !finite(chapter.Start) || !finite(chapter.End) || chapter.Start < 0 || chapter.End <= chapter.Start {
			continue
		}
		name := strings.ToLower(strings.Join(strings.FieldsFunc(chapter.Title, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }), " "))
		kind := ""
		switch {
		case name == "recap" || name == "previously on" || name == "previously in" || strings.HasPrefix(name, "previously on ") || strings.HasPrefix(name, "previously in "):
			kind = "recap"
		case slices.Contains([]string{"opening credits", "opening title", "opening titles", "title sequence", "intro", "theme song"}, name):
			kind = "intro"
		case slices.Contains([]string{"commercial", "commercials", "advertisement", "advertisements", "ad break"}, name):
			kind = "commercial"
		case name == "outro" || name == "closing sequence":
			kind = "outro"
		case slices.Contains([]string{"end credits", "closing credits", "credit roll", "credits"}, name):
			kind = "credits"
		}
		if kind != "" {
			markers = append(markers, Marker{Type: kind, Label: strings.ToUpper(kind[:1]) + kind[1:], Start: chapter.Start, End: chapter.End, Source: "chapter"})
		}
	}
	return markers
}

// NormalizeAutoSkip validates and orders selected marker types.
func NormalizeAutoSkip(values []string) ([]string, error) {
	selected := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if !slices.Contains(markerTypes, value) {
			return nil, errors.New("unknown automatic skip marker type")
		}
		selected[value] = true
	}
	result := make([]string, 0, len(selected))
	for _, value := range markerTypes {
		if selected[value] {
			result = append(result, value)
		}
	}
	return result, nil
}

// Types returns all supported marker types in display order.
func Types() []string { return append([]string(nil), markerTypes...) }
