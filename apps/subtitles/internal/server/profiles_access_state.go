package server

import (
	"context"
	"log/slog"
	"path/filepath"
)

const (
	sessionUseWriteSeconds  = 60
	apiKeyUseWriteSeconds   = 3600
	accessStateRetrySeconds = 30
)

func (store *profileStore) saveAccessState(ctx context.Context, operation, file string, now int64) {
	value := any(store.sessions)
	if file == store.apiFile {
		value = store.apiKeys
	}
	document := filepath.Base(file)
	if err := store.persist(file, value); err != nil {
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

func (store *profileStore) retryAccessState(ctx context.Context, now int64) {
	for file, retryAt := range store.accessRetry {
		if retryAt <= now {
			store.saveAccessState(ctx, "retry", file, now)
		}
	}
}
