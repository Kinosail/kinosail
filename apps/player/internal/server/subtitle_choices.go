package server

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"golang.org/x/text/language"
)

type subtitleDefaults struct {
	language, mode, audio string
	limited               bool
}

type subtitleCandidate struct {
	track  subtitleTrack
	option playback.SubtitleOption
	stream int
	order  int
}

var subtitleKindRank = map[string]int{"regular": 0, "captions": 1, "forced": 2, "commentary": 3}

func (store *settingsStore) subtitleDefaults(media probeResult, preference string) subtitleDefaults {
	defaults := subtitleDefaults{language: store.subtitleLanguage(), mode: store.subtitleMode(), audio: defaultAudioLanguage(media), limited: store.subtitlePickerLimited()}
	switch preference {
	case "", "auto":
	case "off":
		defaults.mode = "off"
	default:
		if preferred, err := canonicalSubtitleLanguage(preference); err == nil {
			defaults.language = preferred
		}
	}
	return defaults
}

func defaultAudioLanguage(media probeResult) string {
	for _, track := range media.AudioFacts {
		if track.Default {
			return track.Language
		}
	}
	if len(media.AudioFacts) > 0 {
		return media.AudioFacts[0].Language
	}
	return ""
}

func playbackSubtitleChoices(item library.Item, media probeResult, defaults subtitleDefaults) []subtitleTrack {
	candidates := orderedSubtitleCandidates(item, media, defaults.language)
	if defaults.limited {
		candidates = slices.DeleteFunc(candidates, func(candidate subtitleCandidate) bool {
			return candidate.track.Group != "" || !sameSubtitleLanguage(candidate.option.Language, defaults.language)
		})
	}
	selected := defaultSubtitleCandidate(candidates, defaults)
	tracks := make([]subtitleTrack, len(candidates))
	for index, candidate := range candidates {
		tracks[index] = candidate.track
		tracks[index].Default = index == selected
	}
	return tracks
}

func orderedSubtitleCandidates(item library.Item, media probeResult, preferred string) []subtitleCandidate {
	candidates := subtitleCandidates(item, media)
	slices.SortStableFunc(candidates, func(left, right subtitleCandidate) int { return compareSubtitleCandidates(left, right, preferred) })
	seen := map[string]bool{}
	for index := range candidates {
		key := subtitleLanguageKey(candidates[index].option.Language) + "/" + playback.SubtitleKind(candidates[index].option)
		if seen[key] {
			candidates[index].track.Group = "more"
		}
		seen[key] = true
	}
	slices.SortStableFunc(candidates, func(left, right subtitleCandidate) int {
		return strings.Compare(left.track.Group, right.track.Group)
	})
	labelSubtitleCandidates(candidates)
	return candidates
}

func subtitleCandidates(item library.Item, media probeResult) []subtitleCandidate {
	candidates := make([]subtitleCandidate, 0, len(media.SubtitleFacts)+len(item.Subtitles))
	for _, track := range media.SubtitleFacts {
		if !track.Text {
			continue
		}
		role := track.Role
		if role == "translation" {
			role = ""
		}
		option := playback.SubtitleOption{Language: track.Language, Role: role, Title: track.Title, Forced: track.Forced, Converted: track.Converted, Default: track.Default}
		candidates = append(candidates, subtitleCandidate{option: option, stream: track.SourceIndex, order: len(candidates), track: subtitleTrack{Source: fmt.Sprintf("/subtitle/%s/embedded/%d", item.ID, track.SourceIndex), Language: track.Language, Role: role, Kind: subtitleTrackKind(role), Forced: track.Forced, Embedded: true}})
	}
	for index, path := range item.Subtitles {
		role := subtitleRoleFromPath(path)
		forced := strings.Contains(strings.ToLower(filepath.Base(path)), ".forced.")
		code := sidecarSubtitleLanguage(item.Path, path)
		option := playback.SubtitleOption{Language: code, Role: role, Forced: forced, External: true, Default: index == 0}
		candidates = append(candidates, subtitleCandidate{option: option, stream: -1, order: len(candidates), track: subtitleTrack{Source: fmt.Sprintf("/subtitle/%s/%d", item.ID, index), Language: code, Role: role, Kind: subtitleTrackKind(role), Forced: forced}})
	}
	return candidates
}

func subtitleTrackKind(role string) string {
	if role == "captions" {
		return "captions"
	}
	return "subtitles"
}

func compareSubtitleCandidates(left, right subtitleCandidate, preferred string) int {
	if difference := subtitleLanguageRank(left.option.Language, preferred) - subtitleLanguageRank(right.option.Language, preferred); difference != 0 {
		return difference
	}
	if difference := strings.Compare(playback.SubtitleLanguageName(left.option.Language), playback.SubtitleLanguageName(right.option.Language)); difference != 0 {
		return difference
	}
	if difference := subtitleKindRank[playback.SubtitleKind(left.option)] - subtitleKindRank[playback.SubtitleKind(right.option)]; difference != 0 {
		return difference
	}
	if difference := subtitleQuality(right.option) - subtitleQuality(left.option); difference != 0 {
		return difference
	}
	return left.order - right.order
}

func subtitleLanguageRank(code, preferred string) int {
	switch {
	case sameSubtitleLanguage(code, preferred):
		return 0
	case subtitleLanguageKey(code) == "und":
		return 2
	default:
		return 1
	}
}

func subtitleQuality(option playback.SubtitleOption) int {
	quality := 0
	if !option.Converted {
		quality += 4
	}
	if option.External {
		quality += 2
	}
	if option.Default {
		quality++
	}
	return quality
}

func subtitleLanguageKey(code string) string {
	if base, err := language.ParseBase(strings.ToLower(strings.TrimSpace(code))); err == nil && base.String() != "und" {
		return base.String()
	}
	return "und"
}

func labelSubtitleCandidates(candidates []subtitleCandidate) {
	used := map[string]int{}
	for index := range candidates {
		candidate := &candidates[index]
		label := playback.SubtitleLabel(candidate.option)
		if candidate.track.Group == "more" {
			label += map[bool]string{true: " · File", false: " · Built-in"}[candidate.option.External]
		}
		used[label]++
		if used[label] > 1 {
			label += " · " + strconv.Itoa(used[label])
		}
		candidate.track.Label, candidate.track.Detail = label, playback.SubtitleDetail(candidate.option)
	}
}

func defaultSubtitleCandidate(candidates []subtitleCandidate, defaults subtitleDefaults) int {
	if defaults.mode == "off" {
		return -1
	}
	choice := func(kind string, fallback bool) int {
		for index, candidate := range candidates {
			matches := sameSubtitleLanguage(candidate.option.Language, defaults.language) || fallback && subtitleLanguageKey(candidate.option.Language) == "und"
			if candidate.track.Group == "" && matches && playback.SubtitleKind(candidate.option) == kind {
				return index
			}
		}
		return -1
	}
	kinds := []string{"regular", "captions", "forced"}
	if defaults.mode == "auto" && (subtitleLanguageKey(defaults.audio) == "und" || sameSubtitleLanguage(defaults.audio, defaults.language)) {
		kinds = []string{"forced"}
	}
	for _, fallback := range []bool{false, true} {
		for _, kind := range kinds {
			if index := choice(kind, fallback); index >= 0 {
				return index
			}
		}
	}
	return -1
}
