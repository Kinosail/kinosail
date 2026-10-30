package server

import (
	"context"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) embeddedReady() bool {
	return manager.probe != nil && manager.probe.executable != "" && manager.probe.ffmpeg != ""
}

func (manager *subtitleManager) fetchSidecar(ctx context.Context, item library.Item, language string) error {
	ctx, settled, admissionErr := manager.provider.admission.enter(ctx)
	if admissionErr != nil {
		return admissionErr
	}
	defer settled()
	target, err := manager.provider.openSidecar(item, language)
	if err != nil {
		return err
	}
	defer target.close()
	if !manager.embeddedReady() {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	source, score, role := "embedded", 100, ""
	data, found := manager.embeddedSubtitle(ctx, item, language)
	if track, trackFound := manager.embeddedSubtitleTrack(ctx, item, language); found && trackFound {
		role = track.Role
	}
	if !found {
		var track SubtitleFacts
		data, track, found = manager.imageSubtitle(ctx, item, language)
		source, score, role = "ocr", 90, track.Role
	}
	if !found {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	cleaned, err := cleanSubtitle(data)
	if err != nil {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	now := time.Now().Unix()
	record := subtitleRecord{Source: source, Score: score, ReleaseMatch: 1, CheckedAt: now, InstalledAt: now, Cleanup: cleaned.Cleanup, Synchronization: "none", TimingEvidence: "embedded", Managed: true, Role: role}
	if err = manager.provider.retainSubtitleOriginal(cleaned.Original, &record); err != nil {
		return err
	}
	install := sidecarInstall{Key: subtitleRecordKey(item.ID, language), Data: cleaned.Data, Record: record, Action: "added", Reason: map[string]string{"embedded": "embedded", "ocr": "image"}[source]}
	if err = manager.provider.installSidecar(target, install); err != nil { //nolint:contextcheck // Ledger commits must finish after client cancellation.
		return err
	}
	manager.provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, manager.settings.subtitlePreference()), "installed", "", item, time.Now()) //nolint:contextcheck // Ledger commits must finish after client cancellation.
	return nil
}

func (manager *subtitleManager) embeddedSubtitle(ctx context.Context, item library.Item, language string) ([]byte, bool) {
	best, found := manager.embeddedSubtitleTrack(ctx, item, language)
	if !found {
		return nil, false
	}
	path, data, err := manager.probe.extractEmbedded(ctx, item, best.SourceIndex)
	if err != nil {
		return nil, false
	}
	if path != "" {
		data, err = os.ReadFile(path)
	}
	return data, err == nil && len(data) > 0
}

func (manager *subtitleManager) embeddedSubtitleTrack(ctx context.Context, item library.Item, language string) (SubtitleFacts, bool) {
	if !manager.embeddedReady() {
		return SubtitleFacts{}, false
	}
	return manager.embeddedSubtitleFacts(manager.probe.facts(ctx, item), language)
}

func (manager *subtitleManager) embeddedSubtitleFacts(media probeResult, language string) (SubtitleFacts, bool) {
	best, score := SubtitleFacts{}, -1
	preference := manager.preference()
	for _, track := range media.SubtitleFacts {
		if !track.Text || track.Forced || !subtitleLanguageMatches(language, track.Language) || preference == "sdh" && track.Role != "captions" {
			continue
		}
		trackScore := 0
		if track.Role == "translation" {
			trackScore += 2
		}
		if track.Default {
			trackScore++
		}
		if trackScore > score {
			best, score = track, trackScore
		}
	}
	if score < 0 {
		return SubtitleFacts{}, false
	}
	return best, true
}
