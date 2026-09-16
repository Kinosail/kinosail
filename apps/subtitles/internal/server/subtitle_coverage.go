package server

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) searchableSubtitleLanguage(ctx context.Context, item library.Item, languages []string) (string, bool) {
	for _, language := range languages {
		if manager.provider.supports(language) {
			return language, true
		}
		if _, found := manager.embeddedSubtitleTrack(ctx, item, language); found {
			return language, true
		}
	}
	return "", false
}

func (manager *subtitleManager) searchableSubtitleFacts(languages []string, media probeResult) (string, bool) {
	for _, language := range languages {
		if manager.provider.supports(language) {
			return language, true
		}
		if _, found := manager.embeddedSubtitleFacts(media, language); manager.embeddedReady() && found {
			return language, true
		}
	}
	return "", false
}

func (manager *subtitleManager) planCoverageAllFacts(item library.Item, languages []string, media probeResult) (bool, []string, []string) {
	tracks, missing := make([]string, 0), make([]string, 0, len(languages))
	for _, language := range languages {
		ready, current := manager.planCoverageFacts(item, language, media)
		for _, track := range current {
			if !slices.Contains(tracks, track) {
				tracks = append(tracks, track)
			}
		}
		if !ready {
			missing = append(missing, language)
		}
	}
	return len(missing) == 0, tracks, missing
}

func subtitleCoverage(item library.Item, language string) (bool, []string) {
	ready, labels, _ := subtitleCoverageAll(item, []string{canonicalSubtitleLanguage(language)})
	return ready, labels
}

func subtitleCoverageAll(item library.Item, languages []string) (bool, []string, []string) {
	mediaBase := strings.TrimSuffix(item.Path, filepath.Ext(item.Path))
	labels := make([]string, 0, len(item.Subtitles))
	available := make(map[string]int, len(languages))
	reportedLanguages := make([]string, 0, len(item.Subtitles))
	for _, path := range item.Subtitles {
		label, language := subtitleTrackLanguage(path, mediaBase, languages)
		labels = append(labels, label)
		if language != "" {
			available[language]++
			reportedLanguages = append(reportedLanguages, language)
		}
	}
	missing := missingSubtitleLanguages(languages, available, reportedLanguages)
	return len(missing) == 0, labels, missing
}

func subtitleTrackLanguage(path, mediaBase string, languages []string) (string, string) {
	base := strings.TrimSuffix(path, filepath.Ext(path))
	suffix := strings.TrimPrefix(base, mediaBase)
	if suffix == "" {
		if len(languages) > 0 {
			return "Default", languages[0]
		}
		return "Default", ""
	}
	for _, part := range strings.Split(strings.TrimPrefix(suffix, "."), ".") {
		if canonical := canonicalSubtitleLanguage(part); canonical != "" {
			return canonical, canonical
		}
	}
	return "Other", ""
}

func missingSubtitleLanguages(languages []string, available map[string]int, reportedLanguages []string) []string {
	missing := make([]string, 0, len(languages))
	for _, language := range languages {
		if available[language] > 0 {
			available[language]--
			continue
		}
		matched := false
		for _, reported := range reportedLanguages {
			if available[reported] > 0 && subtitleLanguageMatches(language, reported) {
				available[reported]--
				matched = true
				break
			}
		}
		if !matched {
			missing = append(missing, language)
		}
	}
	return missing
}
