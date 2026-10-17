//go:build linux || darwin

package playback

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestWaitHLSChangePollsAndWarnsWhenWatchesAreExhausted(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private-title", "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "segment-00001.m4s")
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	hlsWatchWarned.Store(0)
	var probes atomic.Int64
	probe := func() error { probes.Add(1); _, err := os.Stat(target); return err }
	ready := make(chan bool, 1)
	exhaustFileDescriptors(t, func() {
		go func() { ready <- WaitHLSChange(t.Context(), []string{directory}, nil, probe) }()
		for deadline := time.Now().Add(2 * time.Second); probes.Load() < 2 && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
	})
	publishHLSTestFile(t, target, "segment")
	select {
	case observed := <-ready:
		if !observed || probes.Load() < 3 {
			t.Fatalf("polling fallback = %v after %d probes", observed, probes.Load())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("polling fallback did not observe the published segment")
	}
	output := logs.String()
	if strings.Count(output, "HLS change watch unavailable") != 1 || !strings.Contains(output, "level=WARN") || !strings.Contains(output, "failure_class=instance-limit") || !strings.Contains(output, "outcome=polling") || strings.Contains(output, "private-title") {
		t.Fatalf("watch fallback log = %q", output)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	exhaustFileDescriptors(t, func() {
		if WaitHLSChange(canceled, []string{directory}, nil, func() error { return os.ErrNotExist }) {
			t.Error("canceled fallback wait became ready")
		}
	})
	if strings.Count(logs.String(), "HLS change watch unavailable") != 1 {
		t.Fatalf("repeated watch fallback was not rate limited: %q", logs.String())
	}
}

func exhaustFileDescriptors(t *testing.T, run func()) {
	t.Helper()
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatal(err)
	}
	exhausted := limit
	exhausted.Cur = 0
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &exhausted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}
