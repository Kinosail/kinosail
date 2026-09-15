package viewing

import (
	"context"
	"sort"
	"time"
)

const initialSyncDelay = time.Duration(250_000_000)

func (manager *Manager) schedule(ctx context.Context) {
	delay := initialSyncDelay
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		manager.RunDue(ctx, time.Now().UTC())
		delay = time.Minute
	}
}

// RunDue queues every due sync within the bounded worker capacity.
func (manager *Manager) RunDue(ctx context.Context, now time.Time) {
	manager.mu.Lock()
	manager.expirePreviews(now)
	ids := make([]string, 0, len(manager.syncs))
	for id := range manager.syncs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		sync := manager.syncs[id]
		if sync.Running || sync.Queued || !sync.NextRun.IsZero() && sync.NextRun.After(now) {
			continue
		}
		select {
		case manager.syncSlots <- struct{}{}:
			sync.Queued = true
			manager.syncs[id] = sync
			go func(id string) {
				defer func() { <-manager.syncSlots }()
				_, _ = manager.runReserved(ctx, id)
			}(id)
		default:
			manager.syncDeferred.Add(1)
		}
	}
	manager.mu.Unlock()
}

func (manager *Manager) expirePreviews(now time.Time) {
	manager.engine.Store.ExpireLocked(now)
}

func (manager *Manager) deletePreviewLocked(id string) {
	manager.engine.Store.DeleteLocked(id)
}
