package playback

import (
	"context"
	"errors"
	"os"
	"time"
)

// WaitHLSReady follows Player's readiness cadence without retrying permanent errors.
func WaitHLSReady(ctx context.Context, probe func() error) bool {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := probe(); err == nil {
			return true
		} else if !errors.Is(err, os.ErrNotExist) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
