package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	sharedmaintenance "github.com/MikeO7/kinosail/packages/maintenance"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestSubtitlePreparationLeavesABackgroundSlotForOtherWork(t *testing.T) {
	root := t.TempDir()
	started, release := filepath.Join(root, "started"), filepath.Join(root, "release")
	ffprobe, ffmpeg := filepath.Join(root, "ffprobe"), filepath.Join(root, "ffmpeg")
	for path, content := range map[string]string{
		ffprobe: "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":2,\"codec_type\":\"subtitle\",\"codec_name\":\"subrip\",\"tags\":{\"language\":\"eng\"}}],\"format\":{\"duration\":\"30\"}}'\n",
		ffmpeg:  "#!/bin/sh\nprintf x >> '" + started + "'\nwhile [ ! -f '" + release + "' ]; do sleep 0.01; done\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:02.000\\nCaptions\\n'\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil { //nolint:gosec // Test executables must be runnable.
			t.Fatal(err)
		}
	}
	items := make([]library.Item, 6)
	for index := range items {
		path := filepath.Join(root, strconv.Itoa(index)+".mkv")
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
		items[index] = library.Item{ID: strconv.Itoa(index), Kind: "video", Path: path}
	}
	workloads := workload.New(4)
	if workloads.Metrics().BackgroundCapacity != 2 {
		t.Fatalf("fixture background capacity = %d", workloads.Metrics().BackgroundCapacity)
	}
	probe := newMediaProbe(ffprobe)
	probe.ffmpeg, probe.cacheDir = ffmpeg, filepath.Join(root, "cache")
	preparation := newSubtitlePreparation(probe, func() string { return "en" }, workloads, sharedmaintenance.New(t.Context(), time.Hour, 0, sharedmaintenance.Dependencies{
		PruneCache: func(int64) (int64, error) { return 0, nil }, PruneMetadata: func() error { return nil },
	}))
	done := make(chan struct{})
	go func() { preparation.prepare(t.Context(), items); close(done) }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if data, err := os.ReadFile(started); err == nil && len(data) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("subtitle preparation did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	other, err := workloads.Acquire(ctx, workload.Background)
	if err != nil {
		t.Fatalf("another background task waited behind subtitle preparation: %v (active %d)", err, workloads.Metrics().ActiveBackground)
	}
	other()
	if err = os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	<-done
	for _, item := range items {
		if !probe.subtitlePrepared(item, "en") {
			t.Fatalf("item %s was not prepared", item.ID)
		}
	}
}
