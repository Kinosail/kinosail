package server

import (
	"context"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleMaintenanceResult struct {
	Attempted  int `json:"attempted"`
	Added      int `json:"added"`
	Upgraded   int `json:"upgraded"`
	Failed     int `json:"failed"`
	NextCursor int `json:"-"`
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
	for offset := 0; offset < pairCount && result.Attempted < limit; offset++ {
		position := (start + offset) % pairCount
		item := items[position/len(languages)]
		language := languages[position%len(languages)]
		result.NextCursor = (position + 1) % pairCount
		result.add(manager.maintainLanguageItem(ctx, item, language))
	}
	return result
}

func (result *subtitleMaintenanceResult) add(outcome subtitleMaintenanceResult) {
	result.Attempted += outcome.Attempted
	result.Added += outcome.Added
	result.Upgraded += outcome.Upgraded
	result.Failed += outcome.Failed
}

func (manager *subtitleManager) maintainLanguageItem(ctx context.Context, item library.Item, language string) subtitleMaintenanceResult {
	if item.Kind != "video" {
		return subtitleMaintenanceResult{}
	}
	ready, tracks := manager.planCoverage(ctx, item, language)
	sidecarReady, _ := manager.sidecarPlanCoverage(item, language)
	if ready {
		return manager.maintainReadySubtitle(ctx, item, language, tracks, sidecarReady)
	}
	return manager.maintainMissingSubtitle(ctx, item, language)
}

func (manager *subtitleManager) maintainMissingSubtitle(ctx context.Context, item library.Item, language string) subtitleMaintenanceResult {
	_, searchable := manager.searchableSubtitleLanguage(ctx, item, []string{language})
	searchReady := manager.provider.ledger.automaticSearchReady(subtitleSearchKey(item.ID, language, manager.provider.preference()), time.Now())
	if !searchable || !searchReady {
		return subtitleMaintenanceResult{}
	}
	return manager.fetchMaintenanceSubtitle(ctx, item, language)
}

func (manager *subtitleManager) maintainReadySubtitle(ctx context.Context, item library.Item, language string, tracks []string, sidecarReady bool) subtitleMaintenanceResult {
	if !sidecarReady && subtitleTrackListed(tracks, "Embedded "+language) {
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
