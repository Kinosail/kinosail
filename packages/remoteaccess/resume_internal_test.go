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
	var first net.Listener
	select {
	case first = <-listeners:
	case <-time.After(2 * time.Second):
		t.Fatal("public listener did not start")
	}
	waitRemoteState(t, manager, "ready")
	if err := manager.Kill(); err != nil || manager.ResetKill() != nil {
		t.Fatalf("kill/reset: %v", err)
	}
	select {
	case second := <-listeners:
		if second == first {
			t.Fatal("public listener was reused after kill")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("public listener did not resume")
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

func waitRemoteState(t *testing.T, manager *Manager, want string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if manager.Status().State == want {
			return
		}
	}
	t.Fatalf("remote status = %#v, want %s", manager.Status(), want)
}
