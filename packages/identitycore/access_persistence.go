package identitycore

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
)

const AccessRetrySeconds = 30

type AccessPersistence struct {
	mu    sync.Mutex
	retry map[string]int64
}

func (persistence *AccessPersistence) Save(ctx context.Context, persist func(string, any) error, requestID, operation, file string, value any, now int64) {
	document := filepath.Base(file)
	err := persist(file, value)
	persistence.mu.Lock()
	defer persistence.mu.Unlock()
	if err != nil {
		if persistence.retry == nil {
			persistence.retry = make(map[string]int64, 2)
		}
		persistence.retry[file] = now + AccessRetrySeconds
		slog.WarnContext(ctx, "Access state write failed", "request_id", requestID, "operation", operation, "outcome", "retry-scheduled", "document", document, "error", err)
		return
	}
	if _, pending := persistence.retry[file]; pending {
		delete(persistence.retry, file)
		slog.InfoContext(ctx, "Access state write recovered", "request_id", requestID, "operation", operation, "outcome", "persisted", "document", document)
	}
}

func (persistence *AccessPersistence) Due(now int64, sessionFile, apiFile string, sessionsUsed, keysUsed bool) map[string]string {
	persistence.mu.Lock()
	writes := make(map[string]string, 2)
	for file, retryAt := range persistence.retry {
		if retryAt <= now {
			writes[file] = "retry"
		}
	}
	persistence.mu.Unlock()
	if sessionsUsed && writes[sessionFile] == "" {
		writes[sessionFile] = "session-use"
	}
	if keysUsed && writes[apiFile] == "" {
		writes[apiFile] = "api-key-use"
	}
	return writes
}

func (persistence *AccessPersistence) Pending() int {
	persistence.mu.Lock()
	defer persistence.mu.Unlock()
	return len(persistence.retry)
}

func (persistence *AccessPersistence) ExpireRetries(now int64) {
	persistence.mu.Lock()
	defer persistence.mu.Unlock()
	for file := range persistence.retry {
		persistence.retry[file] = now
	}
}
