package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestOfflineDownloadsReserveCapacityForPlayback(t *testing.T) {
	root, tools := t.TempDir(), t.TempDir()
	starts, release, executable := filepath.Join(tools, "starts"), filepath.Join(tools, "release"), filepath.Join(tools, "ffmpeg")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'start\\n' >> %q\nwhile [ ! -f %q ]; do sleep 0.01; done\nfor output; do :; done\nprintf media > \"$output\"\n", starts, release)
	if err := os.WriteFile(executable, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executable, 0o700); err != nil { //nolint:gosec // The temporary fixture must be executable.
		t.Fatal(err)
	}
	workloads := workload.New(workload.HeavyCapacity())
	manager := newDownloadManager(t.Context(), root, executable, newSettingsStore("", "", "", nil), workloads)
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		waitForDownloads(manager)
	})
	profile := viewerProfile{ID: "viewer"}
	for position := range 3 {
		id := strconv.Itoa(position)
		item := library.Item{ID: id, Kind: "audio", Title: "Track", Path: filepath.Join(root, id+".flac"), Added: time.Now()}
		if _, err := manager.Start(profile.ID, item, "audio"); err != nil {
			t.Fatal(err)
		}
	}
	want := workloads.Metrics().BackgroundCapacity
	for deadline := time.Now().Add(3 * time.Second); startedLinesForDownload(starts) < want && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	time.Sleep(100 * time.Millisecond)
	if started := startedLinesForDownload(starts); started != want {
		t.Fatalf("concurrent offline downloads = %d, want %d", started, want)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForDownloads(manager)
}

func TestOfflineDownloadQueueRejectsOverflowBeforeCreatingAJob(t *testing.T) {
	root, tools := t.TempDir(), t.TempDir()
	starts, release, executable := filepath.Join(tools, "starts"), filepath.Join(tools, "release"), filepath.Join(tools, "ffmpeg")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'start\\n' >> %q\nwhile [ ! -f %q ]; do sleep 0.01; done\nfor output; do :; done\nprintf media > \"$output\"\n", starts, release)
	if err := os.WriteFile(executable, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executable, 0o700); err != nil { //nolint:gosec // The temporary fixture must be executable.
		t.Fatal(err)
	}
	manager := newDownloadManager(t.Context(), root, executable, newSettingsStore("", "", "", nil), workload.New(2))
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		waitForDownloads(manager)
	})
	profile := viewerProfile{ID: "viewer"}
	rejected := 0
	for position := range downloadQueueCapacity + 2 {
		id := "queue-" + strconv.Itoa(position)
		item := library.Item{ID: id, Kind: "audio", Title: "Track", Path: filepath.Join(root, id+".flac"), Added: time.Now()}
		if _, err := manager.Start(profile.ID, item, "audio"); err != nil {
			rejected++
		}
	}
	jobs := len(manager.List(profile.ID))
	if rejected != 1 || jobs != downloadQueueCapacity+1 {
		t.Fatalf("rejected = %d, jobs = %d", rejected, jobs)
	}
}

func startedLinesForDownload(path string) int {
	data, _ := os.ReadFile(path)
	return strings.Count(string(data), "start\n")
}

func waitForDownloads(manager *downloadManager) {
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		preparing := false
		for _, job := range manager.List("viewer") {
			preparing = preparing || job.State == "preparing"
		}
		if !preparing {
			return
		}
	}
}
