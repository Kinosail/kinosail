package server

import (
	"context"
	"sync"
	"time"
)

const (
	subSourceCacheLifetime = time.Hour
	subSourceCacheLimit    = 256
)

type subSourceCacheEntry[T any] struct {
	value   T
	expires time.Time
}

type subSourceCache[T any] struct {
	mu      sync.Mutex
	entries map[string]subSourceCacheEntry[T]
}

func (cache *subSourceCache[T]) get(key string, now time.Time) (T, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, found := cache.entries[key]
	if !found || !now.Before(entry.expires) {
		var empty T
		return empty, false
	}
	return entry.value, true
}

func (cache *subSourceCache[T]) put(key string, value T, now time.Time) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil {
		cache.entries = make(map[string]subSourceCacheEntry[T])
	}
	if _, found := cache.entries[key]; !found && len(cache.entries) >= subSourceCacheLimit {
		oldest := ""
		for existing, entry := range cache.entries {
			if !now.Before(entry.expires) {
				delete(cache.entries, existing)
				continue
			}
			if oldest == "" || entry.expires.Before(cache.entries[oldest].expires) {
				oldest = existing
			}
		}
		if len(cache.entries) >= subSourceCacheLimit {
			delete(cache.entries, oldest)
		}
	}
	cache.entries[key] = subSourceCacheEntry[T]{value: value, expires: now.Add(subSourceCacheLifetime)}
}

func cachedSubSourceJSON[T any](ctx context.Context, provider *subSourceProvider, cache *subSourceCache[T], route, endpoint string) (T, error) {
	if value, found := cache.get(endpoint, provider.now()); found {
		return value, nil
	}
	var value T
	if err := provider.requestJSON(ctx, route, endpoint, &value); err != nil {
		return value, err
	}
	cache.put(endpoint, value, provider.now())
	return value, nil
}
