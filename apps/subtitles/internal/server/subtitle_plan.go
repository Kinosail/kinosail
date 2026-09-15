package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) planCoverage(ctx context.Context, item library.Item, language string) (bool, []string) {
	var media probeResult
	if manager.probe != nil {
		media = manager.probe.facts(ctx, item)
	}
	return manager.planCoverageFacts(item, language, media)
}

func (manager *subtitleManager) planCoverageFacts(item library.Item, language string, media probeResult) (bool, []string) { //nolint:cyclop,gocognit // This projection keeps embedded, sidecar, and audio plan rules together.
	ready, tracks := manager.sidecarPlanCoverage(item, language)
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
	ready, tracks := subtitleCoverage(item, language)
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
		if err == nil && found && record.Role == "captions" {
			data, readErr := readUpgradeSidecar(subtitleSidecarPath(item, language))
			if readErr == nil && record.Fingerprint == subtitleFingerprint(data) {
				return true, tracks
			}
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
