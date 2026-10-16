package server

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleFactsRefreshProbesInParallelAndBacksOffFailures(t *testing.T) {
	t.Parallel()
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("parallel probing needs at least two cores")
	}
	root := t.TempDir()
	calls, priorities := filepath.Join(root, "calls"), filepath.Join(root, "priorities")
	executable := filepath.Join(root, "ffprobe")
	writeSubtitleFactsExecutable(t, executable, "#!/bin/sh\ncut -d ' ' -f 19 /proc/self/stat >> '"+priorities+"' 2>/dev/null\nfor argument in \"$@\"; do last=\"$argument\"; done\nprintf '%s\\n' \"$last\" >> '"+calls+"'\nsleep 0.4\ncase \"$last\" in *broken*) exit 1;; esac\nprintf '%s' '{\"format\":{\"duration\":\"10\"}}'\n")
	items := make([]library.Item, 0, 4)
	for position := range 4 {
		name := fmt.Sprintf("Film%d.mkv", position)
		if position == 0 {
			name = "broken.mkv"
		}
		item := library.Item{ID: fmt.Sprintf("%016x", position+1), Title: name, Kind: "video", Path: filepath.Join(root, name)}
		if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
		items = append(items, scannedSubtitleFactsItem(t, item))
	}
	index := memoryLibraryIndex(items, true)
	settings := newSettingsStore(root, t.TempDir(), nil)
	probe := newMediaProbe(executable)
	probe.cacheDir, probe.ffmpeg = t.TempDir(), "unused"
	manager := newSubtitleManager(index, settings, newSubtitleProvider(SubtitleConfig{}, probe.cacheDir, "", index, settings, ""), probe)
	started := time.Now()
	manager.refreshFacts(t.Context())
	if elapsed := time.Since(started); elapsed >= 1500*time.Millisecond {
		t.Fatalf("four probes took %s; want parallel probing", elapsed)
	}
	if data := subtitleFactsAPI(t, manager); data.Unavailable != 1 || data.Pending != 0 {
		t.Fatalf("refreshed facts = %#v", data)
	}
	manager.refreshFacts(t.Context())
	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if broken := strings.Count(string(log), "broken.mkv"); broken != 1 {
		t.Fatalf("failed probe ran %d times before its retry time", broken)
	}
	requireBackgroundProbes(t, priorities, 4)
}

func requireBackgroundProbes(t *testing.T, log string, want int) {
	t.Helper()
	if runtime.GOOS != "linux" {
		return
	}
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	own, err := strconv.Atoi(strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))[16])
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	priorities := strings.Fields(string(data))
	if len(priorities) != want {
		t.Fatalf("probe priorities = %q, want %d probes", priorities, want)
	}
	for _, value := range priorities {
		if nice, err := strconv.Atoi(value); err != nil || nice < min(19, max(own, 10)) {
			t.Fatalf("scan-time probe nice = %q, server nice = %d", value, own)
		}
	}
}
