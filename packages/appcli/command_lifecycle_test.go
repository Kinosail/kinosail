package appcli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func TestApplicationLifecycleOutlivesShutdownDrain(t *testing.T) { //nolint:paralleltest // The test delivers a process signal to the shared lifecycle.
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	lifecycles := make(chan context.Context, 1)
	started, release := make(chan struct{}), make(chan struct{})
	var drainingErr error
	var lifecycle context.Context
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		drainingErr = lifecycle.Err()
		writer.WriteHeader(http.StatusNoContent)
	})
	build := func(ctx context.Context, _ *remoteaccess.Manager, _ *trustedhttps.Manager) *http.Server {
		lifecycles <- ctx
		return &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	}
	serve := func(server *http.Server, _ servertransport.TLSConfig) error { return server.Serve(listener) }
	exit := make(chan int, 1)
	go func() {
		exit <- run(testSettings{"remote.mode": "off"}, []os.Signal{syscall.SIGUSR1}, build, func(next http.Handler) http.Handler { return next }, //nolint:contextcheck // The lifecycle under test is created by run itself.
			func(Settings) (*remoteaccess.Manager, error) { return nil, nil },
			func(Settings) (*trustedhttps.Manager, error) { return nil, nil }, serve)
	}()
	lifecycle = <-lifecycles
	response := make(chan error, 1)
	go func() {
		reply, getErr := http.Get("http://" + listener.Addr().String() + "/progress") //nolint:noctx // The request observes the drain from outside the lifecycle.
		if getErr == nil {
			_ = reply.Body.Close()
			if reply.StatusCode != http.StatusNoContent {
				getErr = errors.New(reply.Status)
			}
		}
		response <- getErr
	}()
	<-started
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	waitForListenerClose(t, listener.Addr().String())
	if lifecycle.Err() != nil {
		t.Fatal("lifecycle ended before the HTTP drain completed")
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatalf("in-flight request during drain: %v", err)
	}
	if code := <-exit; code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if drainingErr != nil || !errors.Is(lifecycle.Err(), context.Canceled) {
		t.Fatalf("lifecycle during drain = %v, after shutdown = %v", drainingErr, lifecycle.Err())
	}
}

func waitForListenerClose(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(t.Context(), "tcp", address)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("server did not stop accepting connections after the signal")
}
