package mediaprobe

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const healthyProbeOutput = `{"streams":[{"codec_type":"video","codec_name":"h264"}],"format":{"format_name":"mp4","duration":"120"}}`

func transientProbeItem(t *testing.T, script func(root string) string) (*Probe, library.Item, string) {
	t.Helper()
	root := t.TempDir()
	media, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, script(root))
	return New(executable), library.Item{ID: "film", Kind: "video", Path: media}, root
}

func TestCanceledProbeLeaderDoesNotGiveLiveWaitersEmptyFacts(t *testing.T) {
	probe, item, root := transientProbeItem(t, func(root string) string {
		return "#!/bin/sh\nprintf x >> '" + filepath.Join(root, "started") + "'\nsleep 0.5\nprintf '%s' '" + healthyProbeOutput + "'\n"
	})
	background, cancel := context.WithCancel(t.Context())
	defer cancel()
	leader := make(chan Result, 1)
	go func() { leader <- probe.Facts(background, item) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background probe did not start")
		}
		time.Sleep(time.Millisecond)
	}
	waiter := make(chan Result, 1)
	go func() { waiter <- probe.Facts(t.Context(), item) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if result := <-leader; result.Video.Codec != "" || !result.Interrupted {
		t.Errorf("canceled background probe = %#v", result)
	}
	if result := <-waiter; result.Video.Codec != "h264" || result.Duration != 120 || result.Interrupted {
		t.Fatalf("interactive waiter codec=%q duration=%v interrupted=%t", result.Video.Codec, result.Duration, result.Interrupted)
	}
}

func TestKilledProbeDoesNotPinEmptyFacts(t *testing.T) {
	probe, item, root := transientProbeItem(t, func(root string) string {
		killed := filepath.Join(root, "killed")
		return "#!/bin/sh\nif [ ! -e '" + killed + "' ]; then : > '" + killed + "'; kill -KILL $$; fi\nprintf '%s' '" + healthyProbeOutput + "'\n"
	})
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	if result := probe.Facts(t.Context(), item); result.Video.Codec != "" || !result.Interrupted {
		t.Errorf("killed probe = %#v", result)
	}
	failure := logs.String()
	if !strings.Contains(failure, `"level":"WARN","msg":"media probe failed","operation":"mediaprobe.facts","item_id":"film","outcome":"interrupted"`) || strings.Contains(failure, root) {
		t.Errorf("killed probe log = %s", failure)
	}
	if result := probe.Facts(t.Context(), item); result.Video.Codec != "h264" || result.Duration != 120 {
		t.Fatalf("probe after a killed attempt codec=%q duration=%v", result.Video.Codec, result.Duration)
	}
	if logs.String() != failure {
		t.Fatalf("successful probe logged %s", strings.TrimPrefix(logs.String(), failure))
	}
}
