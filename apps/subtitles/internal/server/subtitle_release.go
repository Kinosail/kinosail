package server

import (
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"strings"
)

var subtitleReleaseGroup = regexp.MustCompile(`(?i)-([a-z0-9]{2,24})(?:\.(?:srt|vtt|ass|ssa|mkv|mp4))?$`)

// Title identity and release timing are separate evidence. A matching title
// alone must never bypass audio verification.
func subtitleReleaseSimilarity(left, right string) float64 {
	score := subtitleTokenSimilarity(left, right)
	l, r := subtitleReleaseParts(left), subtitleReleaseParts(right)
	anchored := false
	for _, key := range []string{"source", "edition", "group"} {
		if l[key] != "" && r[key] != "" {
			if l[key] != r[key] {
				if key == "edition" {
					return min(score, 0.3)
				}
				return min(score, 0.65)
			}
			anchored = true
		} else if l[key] != r[key] && key == "edition" {
			return min(score, 0.65)
		}
	}
	if !anchored {
		return min(score, 0.79)
	}
	return score
}

func subtitleReleaseParts(value string) map[string]string {
	parts := make(map[string]string)
	if match := subtitleReleaseGroup.FindStringSubmatch(value); len(match) > 1 {
		if !oneOf(strings.ToLower(match[1]), "dl", "rip", "ray") {
			parts["group"] = strings.ToLower(match[1])
		}
	}
	words := " " + strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(value)) + " "
	for _, source := range []struct {
		name    string
		aliases []string
	}{
		{"bluray", []string{"bluray", "blu ray", "bdrip", "brrip"}},
		{"web", []string{"web", "webdl", "webrip"}},
		{"dvd", []string{"dvd", "dvdrip"}},
		{"broadcast", []string{"hdtv", "pdtv"}},
	} {
		for _, alias := range source.aliases {
			if strings.Contains(words, " "+alias+" ") {
				parts["source"] = source.name
			}
		}
	}
	for _, edition := range []string{"extended", "unrated", "uncut", "theatrical", "directors cut", "director's cut", "remastered"} {
		if strings.Contains(words, " "+edition+" ") {
			parts["edition"] += edition + ";"
		}
	}
	return parts
}

var subtitleReleaseResolution = regexp.MustCompile(`(?i)^\d{3,4}[pi]$`)

// Clean only a filename-derived movie title with recognizable release metadata.
// Curated titles remain authoritative; release details still rank separately.
func subtitleSearchIdentity(item library.Item) (string, string) {
	base := strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path))
	words := func(value string) []string {
		return strings.Fields(strings.NewReplacer(".", " ", "_", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ").Replace(value))
	}
	if item.LocalTitle || !strings.EqualFold(strings.Join(words(item.Title), " "), strings.Join(words(base), " ")) || subtitleReleaseParts(base)["source"] == "" {
		return item.Title, item.Year
	}
	parts := words(base)
	end := len(parts)
	for index, word := range parts {
		if subtitleReleaseResolution.MatchString(word) || oneOf(strings.ToLower(word), "bluray", "blu", "bdrip", "brrip", "web", "webdl", "webrip", "dvd", "dvdrip", "hdtv", "pdtv", "x264", "x265", "h264", "h265", "hevc") {
			end = index
			break
		}
	}
	parts = parts[:end]
	for len(parts) > 1 && oneOf(strings.ToLower(parts[len(parts)-1]), "extended", "unrated", "uncut", "theatrical", "remastered", "cut", "directors", "director's") {
		parts = parts[:len(parts)-1]
	}
	year := item.Year
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		number, err := strconv.Atoi(last)
		if err == nil && len(last) == 4 && number >= 1888 && number <= time.Now().Year()+2 && (year == "" || year == last) {
			year = last
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) == 0 {
		return item.Title, item.Year
	}
	return strings.Join(parts, " "), year
}
