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

func (manager *subtitleManager) fetchSidecar(ctx context.Context, item library.Item, language string) error { //nolint:gocognit // Embedded extraction and durable ledger rollback form one atomic operation.
	target, err := manager.provider.openSidecar(item, language)
	if err != nil {
		return err
	}
	defer target.close()
	if !manager.embeddedReady() {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	data, found := manager.embeddedSubtitle(ctx, item, language)
	if !found {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	cleaned, err := cleanSubtitle(data)
	if err != nil {
		return manager.provider.fetchSidecar(ctx, item, language)
	}
	if err = target.write("", cleaned.Data, true); err != nil {
		return err
	}
	now := time.Now().Unix()
	record := target.record(cleaned.Data, subtitleRecord{Source: "embedded", Score: 100, ReleaseMatch: 1, CheckedAt: now, InstalledAt: now, Cleanup: cleaned.Cleanup, Synchronization: "none", TimingEvidence: "embedded", Managed: true})
	if track, found := manager.embeddedSubtitleTrack(ctx, item, language); found {
		record.Role = track.Role
	}
	err = manager.provider.retainSubtitleOriginal(cleaned.Original, &record)
	if err == nil {
		err = manager.provider.ledger.store(subtitleRecordKey(item.ID, language), record)
	}
	if err != nil {
		_ = target.remove()
	} else {
		manager.provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, manager.settings.subtitlePreference()), "installed", "", time.Now())
	}
	return err
}

func (manager *subtitleManager) embeddedSubtitle(ctx context.Context, item library.Item, language string) ([]byte, bool) { //nolint:cyclop // Track selection and probe-cache extraction are one embedded subtitle operation.
	best, found := manager.embeddedSubtitleTrack(ctx, item, language)
	if !found {
		return nil, false
	}
	path, data, err := manager.probe.extractEmbedded(ctx, item, best.SourceIndex)
	if err != nil {
		return nil, false
	}
	if path != "" {
		data, err = os.ReadFile(path) //nolint:gosec // The path is the probe-owned embedded subtitle cache.
	}
	return data, err == nil && len(data) > 0
}

func (manager *subtitleManager) embeddedSubtitleTrack(ctx context.Context, item library.Item, language string) (SubtitleFacts, bool) { //nolint:cyclop // Embedded-track selection validates all media variants together.
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
