package server

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleFactVersion struct {
	path     string
	size     int64
	modified time.Time
}

// Track checks follow scans in one background worker. Browsing never starts or
// waits for FFprobe, including when the persistent cache is cold or stale.
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
	manager.index.AddAnalyzer(func([]library.Item) {
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
	live := make(map[string]bool, len(items))
	for _, item := range items {
		live[item.ID] = true
		if ctx.Err() != nil {
			return
		}
		if item.Kind != "video" {
			continue
		}
		version := subtitleFactVersion{item.Path, item.Size, item.Added}
		manager.probe.facts(ctx, item)
		if ctx.Err() != nil {
			return
		}
		if _, found := manager.probe.core.CachedFacts(item); found {
			manager.factFailures.Delete(item.ID)
		} else {
			manager.factFailures.Store(item.ID, version)
		}
	}
	manager.factFailures.Range(func(key, _ any) bool {
		if !live[key.(string)] {
			manager.factFailures.Delete(key)
		}
		return true
	})
}

func (manager *subtitleManager) cachedFacts(item library.Item) (probeResult, bool, bool) {
	if manager.probe == nil || manager.probe.executable == "" {
		return probeResult{}, true, false
	}
	media, found := manager.probe.core.MemoryScanFacts(item)
	failedVersion, failed := manager.factFailures.Load(item.ID)
	return media, found, !found && failed && failedVersion == (subtitleFactVersion{item.Path, item.Size, item.Added})
}
