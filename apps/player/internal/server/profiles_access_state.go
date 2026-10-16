package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"
)

const (
	accessStateRetrySeconds = 30
	accessFlushInterval     = time.Minute
)

func (store *profileStore) saveAccessState(ctx context.Context, operation, file string, now int64) {
	value := any(store.sessions)
	if file == store.apiFile {
		value = store.apiKeys
	}
	document := filepath.Base(file)
	err := store.persist(file, value)
	store.retryMu.Lock()
	defer store.retryMu.Unlock()
	if err != nil {
		if store.accessRetry == nil {
			store.accessRetry = make(map[string]int64, 2)
		}
		store.accessRetry[file] = now + accessStateRetrySeconds
		slog.WarnContext(ctx, "Access state write failed", "request_id", requestActivityID(ctx), "operation", operation, "outcome", "retry-scheduled", "document", document, "error", err)
		return
	}
	if _, pending := store.accessRetry[file]; pending {
		delete(store.accessRetry, file)
		slog.InfoContext(ctx, "Access state write recovered", "request_id", requestActivityID(ctx), "operation", operation, "outcome", "persisted", "document", document)
	}
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
	writes := store.dueAccessRetries(now)
	if sessionsUsed && writes[store.sessionFile] == "" {
		writes[store.sessionFile] = "session-use"
	}
	if keysUsed && writes[store.apiFile] == "" {
		writes[store.apiFile] = "api-key-use"
	}
	if len(writes) == 0 {
		return
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	for file, operation := range writes {
		store.saveAccessState(ctx, operation, file, now)
	}
}

func (store *profileStore) dueAccessRetries(now int64) map[string]string {
	store.retryMu.Lock()
	defer store.retryMu.Unlock()
	writes := make(map[string]string, 2)
	for file, retryAt := range store.accessRetry {
		if retryAt <= now {
			writes[file] = "retry"
		}
	}
	return writes
}
