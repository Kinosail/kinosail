package server

import (
	"context"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const subtitleMaintenanceParallelism = 3

type subtitleMaintenanceResult struct {
	Attempted  int `json:"attempted"`
	Added      int `json:"added"`
	Upgraded   int `json:"upgraded"`
	Failed     int `json:"failed"`
	NextCursor int `json:"-"`
	changed    []string
}

func (manager *subtitleManager) maintainLanguageItems(ctx context.Context, items []library.Item, languages []string, limit, start int) subtitleMaintenanceResult {
	result := subtitleMaintenanceResult{}
	canonical, err := validateSubtitleLanguages(languages)
	if limit < 1 || len(items) == 0 || err != nil || !manager.embeddedReady() && !manager.provider.configured() {
		return result
	}
	languages = canonical
	pairCount := len(items) * len(languages)
	start %= pairCount
	type outcome struct {
		subtitleMaintenanceResult
		offset   int
		admitted bool
	}
	done := make(chan outcome)
	inflight, busy, offset := 0, pairCount, 0
	collect := func() {
		finished := <-done
		inflight--
		result.add(finished.subtitleMaintenanceResult)
		if !finished.admitted {
			busy = min(busy, finished.offset)
		}
	}
	for ; offset < pairCount && busy == pairCount; offset++ {
		for inflight > 0 && (inflight >= subtitleMaintenanceParallelism || result.Attempted+inflight >= limit) {
			collect()
		}
		if result.Attempted >= limit || busy < pairCount {
			break
		}
		position := (start + offset) % pairCount
		item, language, pairOffset := items[position/len(languages)], languages[position%len(languages)], offset
		inflight++
		go func() {
			pairResult, admitted := manager.maintainAdmittedLanguageItem(ctx, item, language)
			done <- outcome{pairResult, pairOffset, admitted}
		}()
	}
	for inflight > 0 {
		collect()
	}
	result.NextCursor = (start + min(offset, busy)) % pairCount
	return result
}

func (manager *subtitleManager) maintainAdmittedLanguageItem(ctx context.Context, item library.Item, language string) (subtitleMaintenanceResult, bool) {
	ctx, settled, err := manager.provider.admission.enterBackground(ctx)
	if err != nil {
		return subtitleMaintenanceResult{}, false
	}
	defer settled()
	return manager.maintainLanguageItem(ctx, item, language), true
}

func (result *subtitleMaintenanceResult) add(outcome subtitleMaintenanceResult) {
	result.Attempted += outcome.Attempted
	result.Added += outcome.Added
	result.Upgraded += outcome.Upgraded
	result.Failed += outcome.Failed
	result.changed = append(result.changed, outcome.changed...)
}

func (manager *subtitleManager) maintainLanguageItem(ctx context.Context, item library.Item, language string) subtitleMaintenanceResult {
	if item.Kind != "video" {
		return subtitleMaintenanceResult{}
	}
	ready, sidecarReady, tracks := manager.planSidecarCoverage(ctx, item, language)
	var result subtitleMaintenanceResult
	if ready {
		result = manager.maintainReadySubtitle(ctx, item, language, tracks, sidecarReady)
	} else {
		result = manager.maintainMissingSubtitle(ctx, item, language)
	}
	if result.Added+result.Upgraded > 0 {
		result.changed = []string{item.Path}
	}
	return result
}

func (manager *subtitleManager) maintainMissingSubtitle(ctx context.Context, item library.Item, language string) subtitleMaintenanceResult {
	_, searchable := manager.searchableSubtitleLanguage(ctx, item, []string{language})
	searchReady := manager.provider.ledger.automaticSearchReady(subtitleSearchKey(item.ID, language, manager.provider.preference()), item, time.Now())
	if !searchable || !searchReady {
		return subtitleMaintenanceResult{}
	}
	return manager.fetchMaintenanceSubtitle(ctx, item, language)
}

func (manager *subtitleManager) maintainReadySubtitle(ctx context.Context, item library.Item, language string, tracks []string, sidecarReady bool) subtitleMaintenanceResult {
	if !sidecarReady && subtitleTrackListed(tracks, "Embedded "+language) {
		if !manager.provider.ledger.automaticSearchReady(subtitleSearchKey(item.ID, language, manager.provider.preference()), item, time.Now()) {
			return subtitleMaintenanceResult{}
		}
		return manager.fetchMaintenanceSubtitle(ctx, item, language)
	}
	if !manager.provider.upgradeEligible(item, language, time.Now()) {
		return subtitleMaintenanceResult{}
	}
	result := subtitleMaintenanceResult{Attempted: 1}
	if upgraded, err := manager.provider.upgradeSidecar(ctx, item, language); upgraded {
		result.Upgraded = 1
	} else if err != nil && !errors.Is(err, errNoTrustedSubtitle) {
		result.Failed = 1
	}
	return result
}

func (manager *subtitleManager) fetchMaintenanceSubtitle(ctx context.Context, item library.Item, language string) subtitleMaintenanceResult {
	if manager.fetchSidecar(ctx, item, language) == nil {
		return subtitleMaintenanceResult{Attempted: 1, Added: 1}
	}
	return subtitleMaintenanceResult{Attempted: 1, Failed: 1}
}

func subtitleTrackListed(tracks []string, wanted string) bool {
	for _, track := range tracks {
		if track == wanted {
			return true
		}
	}
	return false
}
