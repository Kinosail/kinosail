package mediaprobe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type probeFixture struct {
	media, calls, frames, executable string
}

func newProbeFixture(t *testing.T, framesExit string) probeFixture {
	t.Helper()
	root := t.TempDir()
	fixture := probeFixture{filepath.Join(root, "film.mp4"), filepath.Join(root, "calls"), filepath.Join(root, "frames"), filepath.Join(root, "ffprobe")}
	if err := os.WriteFile(fixture.media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	framesExit = strings.ReplaceAll(framesExit, "FRAMES", fixture.frames)
	script := "#!/bin/sh\ncase \"$*\" in\n*show_frames*)\nprintf x >> '" + fixture.frames + "'\n" + framesExit +
		"printf '%s' '{\"frames\":[{\"key_frame\":1,\"best_effort_timestamp_time\":\"10.0\"}]}'\n;;\n*)\nprintf x >> '" + fixture.calls +
		"'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n;;\nesac\n"
	writeProbeScript(t, fixture.executable, script)
	return fixture
}

func (fixture probeFixture) count(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Count(string(data), "x")
}

func markerEnrichment(start, end float64) Enrichment {
	return Enrichment{Markers: func(library.Item, []Marker) []Marker {
		return []Marker{{Type: "intro", Label: "Intro", Start: start, End: end, Source: "manual"}}
	}}
}

func TestProbeRemembersFailedProbesForTheSameSourceVersion(t *testing.T) {
	root := t.TempDir()
	media, calls, executable := filepath.Join(root, "broken.mkv"), filepath.Join(root, "calls"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nexit 1\n")
	probe := New(executable)
	item := library.Item{ID: "broken", Kind: "video", Path: media}
	for range 3 {
		if result := probe.Facts(context.Background(), item); result.Video.Codec != "" || result.Interrupted {
			t.Fatalf("failed probe = %#v", result)
		}
	}
	if data, _ := os.ReadFile(calls); string(data) != "x" {
		t.Fatalf("FFprobe calls for one broken source = %q", data)
	}
	if err := os.WriteFile(media, []byte("replaced media"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe.Facts(context.Background(), item)
	if data, _ := os.ReadFile(calls); string(data) != "xx" {
		t.Fatalf("FFprobe calls after the source changed = %q", data)
	}
}

func TestRandomAccessIsPersistedAndFollowsMarkerBoundaries(t *testing.T) {
	fixture := newProbeFixture(t, "")
	cache := t.TempDir()
	item := library.Item{ID: "film", Kind: "video", Path: fixture.media}
	for range 2 {
		probe := New(fixture.executable)
		probe.cacheDir = cache
		if result := probe.Inspect(context.Background(), item, markerEnrichment(10, 30)); len(result.RandomAccess) != 1 || result.RandomAccess[0] != 10 {
			t.Fatalf("random access = %#v", result.RandomAccess)
		}
	}
	if fixture.count(t, fixture.frames) != 1 || fixture.count(t, fixture.calls) != 1 {
		t.Fatalf("restart repeated probes: frames=%d facts=%d", fixture.count(t, fixture.frames), fixture.count(t, fixture.calls))
	}
	probe := New(fixture.executable)
	probe.cacheDir = cache
	probe.Inspect(context.Background(), item, markerEnrichment(40, 60))
	if fixture.count(t, fixture.frames) != 2 {
		t.Fatalf("new marker boundaries reused stale random access: frames=%d", fixture.count(t, fixture.frames))
	}
}

func TestRandomAccessFailureIsNotCachedAsEmpty(t *testing.T) {
	fixture := newProbeFixture(t, "[ \"$(cat 'FRAMES' 2>/dev/null)\" = x ] && exit 1\n")
	probe := New(fixture.executable)
	item := library.Item{ID: "film", Kind: "video", Path: fixture.media}
	if result := probe.Inspect(context.Background(), item, markerEnrichment(10, 30)); len(result.RandomAccess) != 0 {
		t.Fatalf("failed random access = %#v", result.RandomAccess)
	}
	if result := probe.Inspect(context.Background(), item, markerEnrichment(10, 30)); len(result.RandomAccess) != 1 {
		t.Fatalf("random access after transient failure = %#v", result.RandomAccess)
	}
}
