package server

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

type subtitleFactVersion struct {
	path     string
	size     int64
	modified time.Time
}

type subtitleFactFailure struct {
	version  subtitleFactVersion
	attempts int
	retryAt  time.Time
}

const subtitleFactRetry = 15 * time.Minute

func (manager *subtitleManager) scheduleFacts(ctx context.Context) {
	trigger := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
				manager.refreshFacts(ctx)
			}
		}
	}()
	manager.index.AddAnalyzer(func(items []library.Item) {
		if manager.provider != nil {
			manager.provider.ledger.forgetMissing(items, time.Now())
		}
		select {
		case trigger <- struct{}{}:
		default:
		}
	})
}

func (manager *subtitleManager) refreshFacts(ctx context.Context) {
	if manager.probe == nil || manager.probe.executable == "" {
		return
	}
	items, err := manager.index.Snapshot()
	if err != nil {
		return
	}
	now := time.Now()
	live := make(map[string]bool, len(items))
	pending := make([]library.Item, 0, len(items))
	for _, item := range items {
		live[item.ID] = true
		if failure, failed := manager.factFailure(item); item.Kind == "video" && (!failed || !now.Before(failure.retryAt)) {
			pending = append(pending, item)
		}
	}
	jobs := make(chan library.Item)
	var workers sync.WaitGroup
	for range min(len(pending), subtitleWorkerCount()) {
		workers.Go(func() {
			for item := range jobs {
				manager.refreshItemFacts(workload.WithBackgroundPriority(ctx), item, now)
			}
		})
	}
dispatch:
	for _, item := range pending {
		select {
		case jobs <- item:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		return
	}
	manager.factFailures.Range(func(key, _ any) bool {
		if !live[key.(string)] {
			manager.factFailures.Delete(key)
		}
		return true
	})
}

func (manager *subtitleManager) refreshItemFacts(ctx context.Context, item library.Item, now time.Time) {
	manager.probe.facts(ctx, item)
	if ctx.Err() != nil {
		return
	}
	if _, found := manager.probe.core.CachedFacts(item); found {
		manager.factFailures.Delete(item.ID)
		return
	}
	attempts := 1
	if previous, failed := manager.factFailure(item); failed {
		attempts = min(previous.attempts+1, 16)
	}
	delay := min(subtitleFactRetry<<(attempts-1), 24*time.Hour)
	manager.factFailures.Store(item.ID, subtitleFactFailure{version: subtitleFactVersion{item.Path, item.Size, item.Added}, attempts: attempts, retryAt: now.Add(delay)})
	slog.Debug("subtitle media facts unavailable", "item", item.ID, "attempts", attempts, "retry_after_seconds", int(delay.Seconds()))
}

func (manager *subtitleManager) factFailure(item library.Item) (subtitleFactFailure, bool) {
	value, found := manager.factFailures.Load(item.ID)
	failure, ok := value.(subtitleFactFailure)
	return failure, found && ok && failure.version == subtitleFactVersion{item.Path, item.Size, item.Added}
}

func subtitleWorkerCount() int {
	return max(1, min(runtime.GOMAXPROCS(0), 8))
}

func (manager *subtitleManager) cachedFacts(item library.Item) (probeResult, bool, bool) {
	if manager.probe == nil || manager.probe.executable == "" {
		return probeResult{}, true, false
	}
	media, found := manager.probe.core.MemoryScanFacts(item)
	_, failed := manager.factFailure(item)
	return media, found, !found && failed
}
