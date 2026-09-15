package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleCoverageDoesNotRunPlaybackEnrichment(t *testing.T) {
	t.Parallel()
	var chapterCalls atomic.Int32
	chapters := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chapterCalls.Add(1)
		http.Error(w, "unexpected chapter lookup", http.StatusServiceUnavailable)
	}))
	defer chapters.Close()
	root := t.TempDir()
	calls := filepath.Join(root, "calls")
	executable := filepath.Join(root, "ffprobe")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\nprintf '%s' '{\"format\":{\"duration\":\"120\"},\"streams\":[{\"index\":2,\"codec_type\":\"subtitle\",\"codec_name\":\"subrip\",\"tags\":{\"language\":\"eng\"}}],\"chapters\":[{\"start_time\":\"0\",\"end_time\":\"20\",\"tags\":{\"title\":\"Chapter 1\"}}]}'\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "episode.mkv")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := newMediaProbe(executable)
	probe.ffmpeg = "unused"
	probe.chapters = newChapterProvider(chapters.URL)
	manager := &subtitleManager{probe: probe}
	item := library.Item{ID: "episode", Path: media, Kind: "video", Show: "Show", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
	for range 2 {
		ready, tracks := manager.planCoverage(t.Context(), item, "en")
		if !ready || !strings.Contains(strings.Join(tracks, ","), "Embedded en") {
			t.Fatalf("coverage = %v, %v", ready, tracks)
		}
		if _, found := manager.embeddedSubtitleTrack(t.Context(), item, "en"); !found {
			t.Fatal("embedded subtitle missing")
		}
	}
	if chapterCalls.Load() != 0 {
		t.Fatalf("chapter lookups = %d", chapterCalls.Load())
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 1 || strings.Contains(string(data), "-show_frames") {
		t.Fatalf("unexpected probe work: %s", data)
	}
}
