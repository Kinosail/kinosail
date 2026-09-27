package remoteaccess

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeContinuouslyReopensAfterKillReset(t *testing.T) {
	manager := activeManager(t)
	listeners := make(chan net.Listener, 2)
	manager.operations.listen = func(ctx context.Context, _, _ string) (net.Listener, error) {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
		if err == nil {
			listeners <- listener
		}
		return listener, err
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.ServeContinuously(ctx, http.NotFoundHandler()) }()
	first := nextPublicListener(t, listeners)
	waitRemoteState(t, manager, "ready")
	if err := manager.Kill(); err != nil || manager.ResetKill() != nil {
		t.Fatalf("kill/reset: %v", err)
	}
	if nextPublicListener(t, listeners) == first {
		t.Fatal("public listener was reused after kill")
	}
	waitRemoteState(t, manager, "ready")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("public listener did not stop with its context")
	}
}

func nextPublicListener(t *testing.T, listeners <-chan net.Listener) net.Listener {
	t.Helper()
	select {
	case listener := <-listeners:
		return listener
	case <-time.After(2 * time.Second):
		t.Fatal("public listener did not start")
		return nil
	}
}

func waitRemoteState(t *testing.T, manager *Manager, want string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if manager.Status().State == want {
			return
		}
	}
	t.Fatalf("remote status = %#v, want %s", manager.Status(), want)
}
