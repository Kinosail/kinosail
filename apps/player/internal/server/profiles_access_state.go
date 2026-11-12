package server

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

const accessFlushInterval = time.Minute

func (store *profileStore) accessFlusher() identitycore.AccessFlusher {
	return identitycore.AccessFlusher{
		Lock: &store.mu, Sessions: &store.sessions, APIKeys: &store.apiKeys, Use: &store.accessUse, State: &store.accessState,
		SessionFile: store.sessionFile, APIFile: store.apiFile, Persist: store.persist, RequestID: requestActivityID,
	}
}

func (store *profileStore) saveAccessState(ctx context.Context, operation, file string, now int64) {
	store.accessFlusher().Save(ctx, operation, file, now)
}

func (store *profileStore) runAccessFlush(ctx context.Context, interval time.Duration) {
	store.accessFlusher().Run(ctx, interval)
}
