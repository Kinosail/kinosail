package auth

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

type coverageSetupEntropyBarrier struct {
	reader  io.Reader
	mu      sync.Mutex
	started int
	ready   chan struct{}
}

func (barrier *coverageSetupEntropyBarrier) Read(data []byte) (int, error) {
	barrier.mu.Lock()
	barrier.started++
	if barrier.started == 2 {
		close(barrier.ready)
	}
	barrier.mu.Unlock()
	<-barrier.ready
	return barrier.reader.Read(data)
}

func TestCoverageSetupRechecksOwnerAfterPasswordHashing(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	manager.owner = ownerState{}
	reader := rand.Reader
	rand.Reader = &coverageSetupEntropyBarrier{reader: reader, ready: make(chan struct{})}
	t.Cleanup(func() { rand.Reader = reader })
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := manager.Setup(t.Context(), "Owner", currentTestPassword, "Browser"); results <- err }()
	}
	first, second := <-results, <-results
	if first == nil {
		first, second = second, first
	}
	if !errors.Is(first, ErrAlreadyConfigured) || second != nil {
		t.Fatalf("concurrent setup = %v, %v", first, second)
	}
	if len(manager.sessions) != 1 {
		t.Fatalf("concurrent setup created %d sessions", len(manager.sessions))
	}
	if _, err := manager.Setup(t.Context(), "Owner", currentTestPassword, "Browser"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("repeated setup = %v", err)
	}
}

func TestCoveragePasswordChangeRequiresPresentedBrowser(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	manager.owner.PasswordHash = validPasswordHash(t)
	if err := manager.ChangePassword(t.Context(), currentTestPassword, newTestPassword, newTestPassword, strings.Repeat("a", 64)); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("missing browser session = %v", err)
	}
}
