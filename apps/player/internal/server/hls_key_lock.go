package server

import "context"

type hlsKeyLock struct {
	slot  chan struct{}
	users int
}

func (manager *hlsManager) lockHLSKey(ctx context.Context, key string) (func(), error) {
	manager.mu.Lock()
	if manager.keys == nil {
		manager.keys = make(map[string]*hlsKeyLock)
	}
	lock := manager.keys[key]
	if lock == nil {
		lock = &hlsKeyLock{slot: make(chan struct{}, 1)}
		manager.keys[key] = lock
	}
	lock.users++
	manager.mu.Unlock()
	select {
	case lock.slot <- struct{}{}:
		return func() {
			<-lock.slot
			manager.releaseHLSKey(key, lock)
		}, nil
	case <-ctx.Done():
		manager.releaseHLSKey(key, lock)
		return nil, ctx.Err()
	}
}

func (manager *hlsManager) releaseHLSKey(key string, lock *hlsKeyLock) {
	manager.mu.Lock()
	lock.users--
	if lock.users == 0 && manager.keys[key] == lock {
		delete(manager.keys, key)
	}
	manager.mu.Unlock()
}
