package server

import (
	"context"
	"os"
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) planCoverage(ctx context.Context, item library.Item, language string) (bool, []string) {
	ready, _, tracks := manager.planSidecarCoverage(ctx, item, language)
	return ready, tracks
}

func (manager *subtitleManager) planSidecarCoverage(ctx context.Context, item library.Item, language string) (bool, bool, []string) {
	var media probeResult
	if manager.probe != nil {
		media = manager.probe.facts(ctx, item)
	}
	sidecarReady, tracks := manager.sidecarPlanCoverage(item, language)
	ready, tracks := manager.embeddedPlanCoverage(language, media, sidecarReady, tracks)
	return ready, sidecarReady, tracks
}

func (manager *subtitleManager) planCoverageFacts(item library.Item, language string, media probeResult) (bool, []string) {
	ready, tracks := manager.sidecarPlanCoverage(item, language)
	return manager.embeddedPlanCoverage(language, media, ready, tracks)
}

func (manager *subtitleManager) embeddedPlanCoverage(language string, media probeResult, ready bool, tracks []string) (bool, []string) {
	preference := manager.preference()
	for _, track := range media.SubtitleFacts {
		if !track.Text || canonicalSubtitleLanguage(track.Language) != language || track.Forced {
			continue
		}
		if preference == "standard" || track.Role == "captions" {
			return true, append(tracks, "Embedded "+language)
		}
	}
	return ready, tracks
}

func (manager *subtitleManager) sidecarPlanCoverage(item library.Item, language string) (bool, []string) {
	language = canonicalSubtitleLanguage(language)
	_, tracks, missing := subtitleCoverageAll(item, manager.coverageLanguages(language))
	ready := !slices.Contains(missing, language)
	preference := manager.preference()
	if preference != "sdh" {
		return ready, tracks
	}
	for _, path := range item.Subtitles {
		if subtitleLanguageFromPath(item.Path, path) == language && subtitleRoleFromPath(path) == "captions" {
			return true, tracks
		}
	}
	if manager.provider != nil {
		record, found, err := manager.provider.ledger.record(subtitleRecordKey(item.ID, language))
		if err == nil && found && record.Role == "captions" && recordedSidecarUnchanged(subtitleSidecarPath(item, language), record) {
			return true, tracks
		}
	}
	return false, tracks
}

func (manager *subtitleManager) preference() string {
	preference := "standard"
	if manager.settings != nil {
		preference = manager.settings.subtitlePreference()
	}
	return preference
}

func recordedSidecarUnchanged(path string, record subtitleRecord) bool {
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && record.Size > 0 && info.Size() == record.Size && info.ModTime().UnixNano() == record.Modified {
		return true
	}
	data, err := readUpgradeSidecar(path)
	return err == nil && record.Fingerprint == subtitleFingerprint(data)
}

func (manager *subtitleManager) coverageLanguages(language string) []string {
	languages := []string{language}
	if manager.settings != nil {
		languages = manager.settings.subtitleLanguages()
	}
	if !slices.Contains(languages, language) {
		languages = append(slices.Clone(languages), language)
	}
	return languages
}
