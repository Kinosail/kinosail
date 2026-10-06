package main

import (
	"context"
	"errors"
	"net/http"
)

func (peer *r16ProviderPeer) run() {
	defer close(peer.done)
	if err := peer.server.Serve(peer.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		peer.fail()
	}
}

func (peer *r16ProviderPeer) stop(ctx context.Context) bool {
	peer.stopOnce.Do(func() {
		shutdownErr := peer.server.Shutdown(ctx)
		closeErr := peer.listener.Close()
		select {
		case <-peer.done:
		case <-ctx.Done():
			peer.fail()
			return
		}
		peer.mu.Lock()
		peer.joined = shutdownErr == nil && closeErr == nil && peer.active == 0
		peer.stopped = peer.joined && !peer.bad
		peer.mu.Unlock()
	})
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.stopped
}

func (peer *r16ProviderPeer) settled() bool {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.joined
}
