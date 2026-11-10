//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package playback

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKqueueHostsPollForPublishedSegmentsWithoutAWatcher(t *testing.T) {
	hub := newHLSWatchHub()
	useHLSWatchHub(t, hub)
	directory := filepath.Join(t.TempDir(), "360p")
	target := filepath.Join(directory, "segment-00001.m4s")
	ready := make(chan bool, 1)
	go func() {
		ready <- WaitHLSChange(t.Context(), []string{directory}, nil, func() error { _, err := os.Stat(target); return err })
	}()
	time.Sleep(2 * hlsPollInterval)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	publishHLSTestFile(t, target, "segment")
	select {
	case observed := <-ready:
		if !observed {
			t.Fatal("polling wait ended before the segment was published")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("polling wait did not observe the published segment")
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.watcher != nil || len(hub.watched) != 0 || len(hub.pending) != 0 {
		t.Fatalf("kqueue host opened a file watch: watcher=%v watched=%d pending=%d", hub.watcher != nil, len(hub.watched), len(hub.pending))
	}
}
