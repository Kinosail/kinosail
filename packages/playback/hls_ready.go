package playback

import (
	"context"
	"errors"
	"os"
)

func WaitHLSChange(ctx context.Context, directories []string, done <-chan struct{}, probe func() error) bool {
	if ready, final := probeHLSReady(probe); final {
		return ready
	}
	changes := watchHLSChanges(ctx, directories)
	defer changes.Close()
	for {
		if ready, final := probeHLSReady(probe); final {
			return ready
		}
		select {
		case <-ctx.Done():
			return false
		case <-done:
			done = nil
		case <-changes.changed:
		}
	}
}

func probeHLSReady(probe func() error) (bool, bool) {
	err := probe()
	return err == nil, err == nil || !errors.Is(err, os.ErrNotExist)
}
