package backup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// Cancellation can precede admission, arrive during capacity acquisition, or
// arrive while another archive owns the write lock. None may start a new write.
// Once writing has begun, the atomic recovery operation must finish normally.
func TestManagerCancelledWriteDoesNotCreateBackupDirectory(t *testing.T) {
	for _, duringAcquire := range []bool{false, true} {
		t.Run(map[bool]string{false: "before admission", true: "during acquisition"}[duringAcquire], func(t *testing.T) {
			assertCancelledBackupAdmission(t, duringAcquire)
		})
	}
}

func assertCancelledBackupAdmission(t *testing.T, duringAcquire bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	directory := filepath.Join(t.TempDir(), "backups")
	manager := testManager(t.TempDir(), directory)
	acquired, released := 0, 0
	manager.acquire = func(context.Context) (func(), error) {
		acquired++
		return func() { released++ }, nil
	}
	if duringAcquire {
		manager.acquire = func(context.Context) (func(), error) {
			acquired++
			cancel()
			return func() { released++ }, nil
		}
	} else {
		cancel()
	}
	err := manager.Write(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled write = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cancelled write created backup directory: %v", err)
	}
	if duringAcquire && released != 1 {
		t.Errorf("capacity released %d times, want 1", released)
	}
	if !duringAcquire && acquired != 0 {
		t.Errorf("cancelled write acquired capacity %d times", acquired)
	}
}

func TestManagerCancelledQueuedWritePreservesRecoveryPoint(t *testing.T) {
	manager := testManager(t.TempDir(), t.TempDir())
	manager.retention = 1
	started, finish, reserved := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(finish) })
	var writes, released atomic.Int64
	write := manager.writeEncrypted
	manager.writeEncrypted = func(writer io.Writer, directory, key string) error {
		if writes.Add(1) == 1 {
			close(started)
			<-finish
		}
		return write(writer, directory, key)
	}
	manager.acquire = func(context.Context) (func(), error) {
		close(reserved)
		return func() { released.Add(1) }, nil
	}
	first := make(chan error, 1)
	go func() { first <- manager.WriteNow() }()
	<-started
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	queued := make(chan error, 1)
	go func() { queued <- manager.Write(ctx) }()
	<-reserved
	cancel()
	once.Do(func() { close(finish) })
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-queued; !errors.Is(err, context.Canceled) {
		t.Errorf("queued write = %v, want context.Canceled", err)
	}
	if writes.Load() != 1 || released.Load() != 1 {
		t.Errorf("writes=%d releases=%d, want 1 each", writes.Load(), released.Load())
	}
	if status := manager.Status(); status.LastSuccess == "" || status.LastError != "" {
		t.Fatalf("cancelled write changed successful recovery status: %#v", status)
	}
	if err := manager.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerCancellationAfterWriteStartsFinishesRecoveryPoint(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	manager := testManager(t.TempDir(), t.TempDir())
	write := manager.writeEncrypted
	manager.writeEncrypted = func(writer io.Writer, directory, key string) error {
		cancel()
		return write(writer, directory, key)
	}
	if err := manager.Write(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Verify(); err != nil {
		t.Fatal(err)
	}
}
