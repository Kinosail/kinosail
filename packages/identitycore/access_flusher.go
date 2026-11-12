package identitycore

import (
	"context"
	"sync"
	"time"
)

type AccessFlusher struct {
	Lock                 *sync.RWMutex
	Sessions             *map[string]Session
	APIKeys              *map[string]APIKey
	Use                  *AccessUse
	State                *AccessPersistence
	SessionFile, APIFile string
	Persist              func(string, any) error
	RequestID            func(context.Context) string
}

func (flusher AccessFlusher) Save(ctx context.Context, operation, file string, now int64) {
	value := any(*flusher.Sessions)
	if file == flusher.APIFile {
		value = *flusher.APIKeys
	}
	flusher.State.Save(ctx, flusher.Persist, flusher.RequestID(ctx), operation, file, value, now)
}

func (flusher AccessFlusher) Flush(ctx context.Context) {
	now := time.Now().Unix()
	flusher.Lock.Lock()
	sessionsUsed, keysUsed := flusher.Use.Merge(*flusher.Sessions, *flusher.APIKeys)
	flusher.Lock.Unlock()
	writes := flusher.State.Due(now, flusher.SessionFile, flusher.APIFile, sessionsUsed, keysUsed)
	if len(writes) == 0 {
		return
	}
	flusher.Lock.RLock()
	defer flusher.Lock.RUnlock()
	for file, operation := range writes {
		flusher.Save(ctx, operation, file, now)
	}
}

func (flusher AccessFlusher) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			flusher.Flush(context.WithoutCancel(ctx))
			return
		case <-ticker.C:
			flusher.Flush(ctx)
		}
	}
}
