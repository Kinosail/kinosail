package catalog

import (
	"log/slog"
	"sync"
	"time"
)

const progressDocument = "progress.json"

func (store *RequestProgressStore) Update(key string, change ProgressChange) (PlaybackState, PlaybackState, bool, error) {
	store.persistMu.Lock()
	defer store.persistMu.Unlock()
	if !store.coalescing || !store.entries {
		return store.Storage().commit(key, change)
	}
	store.mu.Lock()
	previous, current, changed, err := progressChange(store.values, key, change)
	_, existed := store.values[key]
	deferred := err == nil && changed && existed && !store.flushFailed && progressHeartbeat(previous, current)
	if deferred {
		store.values[key] = current
	}
	store.mu.Unlock()
	if deferred {
		store.dirty[key] = struct{}{}
	}
	if err != nil || !changed || deferred {
		return previous, current, changed, err
	}
	if err := store.database.SaveEntry(progressDocument, key, current); err != nil {
		return previous, current, false, err
	}
	store.mu.Lock()
	store.values[key] = current
	store.mu.Unlock()
	delete(store.dirty, key)
	return previous, current, true, nil
}

func progressHeartbeat(previous, current PlaybackState) bool {
	return current.Seconds > 0 && current.Seconds >= previous.Seconds && current.Watched == previous.Watched &&
		current.Dismissed == previous.Dismissed && current.ReaderPage == previous.ReaderPage &&
		current.ReaderOffset == previous.ReaderOffset && current.Session == previous.Session
}

func (store *RequestProgressStore) StartWriteCoalescing(interval time.Duration) func() {
	store.persistMu.Lock()
	if !store.entries || store.coalescing || interval <= 0 {
		store.persistMu.Unlock()
		return func() {}
	}
	store.coalescing, store.flushFailed, store.dirty = true, false, make(map[string]struct{})
	store.persistMu.Unlock()
	done := make(chan struct{})
	var flusher sync.WaitGroup
	flusher.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				store.persistMu.Lock()
				_ = store.flushLocked("interval")
				store.persistMu.Unlock()
			}
		}
	})
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			flusher.Wait()
			store.persistMu.Lock()
			defer store.persistMu.Unlock()
			_ = store.flushLocked("shutdown")
			store.coalescing = false
		})
	}
}

func (store *RequestProgressStore) Flush() error {
	store.persistMu.Lock()
	defer store.persistMu.Unlock()
	return store.flushLocked("stop")
}

func (store *RequestProgressStore) flushLocked(trigger string) error {
	if len(store.dirty) == 0 {
		return nil
	}
	pending := make(map[string]any, len(store.dirty))
	store.mu.RLock()
	for key := range store.dirty {
		pending[key] = store.values[key]
	}
	store.mu.RUnlock()
	err := store.database.SaveEntries(progressDocument, pending, nil)
	store.flushFailed = err != nil
	if err != nil {
		slog.Warn("Progress flush failed", "operation", "progress.flush", "trigger", trigger, "pending", len(pending), "outcome", "retained-in-memory", "error", err)
		return err
	}
	clear(store.dirty)
	return nil
}
