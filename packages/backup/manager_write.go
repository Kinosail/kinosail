package backup

import (
	"context"
	"time"
)

// Write waits for background capacity and writes one backup.
func (manager *Manager) Write(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := manager.validate(); err != nil {
		manager.recordError(err)
		return err
	}
	release, err := manager.reserve(ctx)
	if err != nil {
		return err
	}
	defer release()
	return manager.write(ctx)
}

func (manager *Manager) reserve(ctx context.Context) (func(), error) {
	if manager.acquire == nil {
		return func() {}, nil
	}
	return manager.acquire(ctx)
}

// WriteNow creates, verifies, and retains one encrypted backup immediately.
func (manager *Manager) WriteNow() error {
	return manager.write(context.Background())
}

func (manager *Manager) write(ctx context.Context) error {
	if err := manager.validate(); err != nil {
		manager.recordError(err)
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	err := manager.writeLocked()
	if err == nil {
		manager.lastSuccess = time.Now().UTC()
		manager.lastVerify, manager.lastError = manager.lastSuccess, ""
	}
	if err != nil {
		manager.lastError = err.Error()
	}
	return err
}
