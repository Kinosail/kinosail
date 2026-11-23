package appcli

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

type shutdownRequest struct {
	once sync.Once
	done chan struct{}
}

var processShutdown = &shutdownRequest{done: make(chan struct{})}

func RequestShutdown() {
	processShutdown.request()
}

func StopWhenClosed(input io.Reader) {
	shutdown := processShutdown
	go func() {
		_, _ = io.Copy(io.Discard, input)
		slog.Info("launcher closed; stopping", "operation", "shutdown", "outcome", "requested")
		shutdown.request()
	}()
}

func (shutdown *shutdownRequest) request() {
	shutdown.once.Do(func() { close(shutdown.done) })
}

func withShutdownRequests(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	requested := processShutdown.done
	go func() {
		select {
		case <-requested:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
