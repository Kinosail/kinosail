package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	subtitleAutomationInterval = 15 * time.Minute
	subtitleAutomationLimit    = 10
)

func (manager *subtitleManager) schedule(ctx context.Context) {
	manager.scheduleFacts(ctx)
	go func() {
		<-ctx.Done()
		manager.drafts.Lock()
		defer manager.drafts.Unlock()
		manager.drafts.closed = true
		if manager.drafts.current.cancel != nil {
			manager.drafts.current.cancel()
		}
	}()
	trigger := make(chan struct{}, 1)
	go manager.runAutomation(ctx, trigger)
	trigger <- struct{}{}
	manager.index.AddAnalyzer(func([]library.Item) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	})
}

func (manager *subtitleManager) runAutomation(ctx context.Context, trigger <-chan struct{}) {
	ticker := time.NewTicker(subtitleAutomationInterval)
	defer ticker.Stop()
	next := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
		case <-ticker.C:
		}
		if delay := time.Until(next); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		result := manager.automate(ctx, subtitleAutomationLimit)
		next = time.Now().Add(subtitleAutomationInterval)
		if result.Attempted > 0 {
			slog.Info("automatic subtitle maintenance completed", "attempted", result.Attempted, "added", result.Added, "upgraded", result.Upgraded)
		}
	}
}

func (manager *subtitleManager) automate(ctx context.Context, limit int) subtitleMaintenanceResult {
	if limit < 1 || manager.readiness().State != "Ready" || !manager.embeddedReady() && !manager.provider.configured() {
		return subtitleMaintenanceResult{}
	}
	items, err := manager.index.Snapshot()
	if err != nil || len(items) == 0 {
		return subtitleMaintenanceResult{}
	}
	result := manager.maintainLanguageItems(ctx, items, manager.settings.subtitleLanguages(), limit, manager.automationCursor)
	manager.automationCursor = result.NextCursor
	return result
}
