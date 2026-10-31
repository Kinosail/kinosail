package server

import (
	"context"
	"time"
)

const accessFlushInterval = time.Minute

func (store *profileStore) saveAccessState(ctx context.Context, operation, file string, now int64) {
	value := any(store.sessions)
	if file == store.apiFile {
		value = store.apiKeys
	}
	store.accessState.Save(ctx, store.persist, requestActivityID(ctx), operation, file, value, now)
}

func (store *profileStore) runAccessFlush(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			store.flushAccessState(context.WithoutCancel(ctx))
			return
		case <-ticker.C:
			store.flushAccessState(ctx)
		}
	}
}

func (store *profileStore) flushAccessState(ctx context.Context) {
	now := time.Now().Unix()
	store.mu.Lock()
	sessionsUsed, keysUsed := store.accessUse.Merge(store.sessions, store.apiKeys)
	store.mu.Unlock()
	writes := store.accessState.Due(now, store.sessionFile, store.apiFile, sessionsUsed, keysUsed)
	if len(writes) == 0 {
		return
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	for file, operation := range writes {
		store.saveAccessState(ctx, operation, file, now)
	}
}
