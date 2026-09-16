package mediaprobe

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProbeEnrichmentAndRandomAccessUseOneSharedCache(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	executable := filepath.Join(root, "ffprobe")
	calls := filepath.Join(root, "calls")
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"frames\":[{\"key_frame\":1,\"best_effort_timestamp_time\":\"10.5\"},{\"key_frame\":0,\"best_effort_timestamp_time\":\"11\"},{\"key_frame\":1,\"best_effort_timestamp_time\":\"bad\"}]}'\n")
	probe := New(executable)
	item := library.Item{ID: "film", Kind: "video", Path: filepath.Join(root, "film.mp4")}
	probe.cache[item.ID] = Result{Duration: 120, Chapters: []Chapter{{Index: 0, Start: 0, End: 20, Title: "Chapter 1"}}}
	enrichment := Enrichment{
		Chapters: func(context.Context, library.Item, float64, []Chapter) []Chapter {
			return []Chapter{{Index: 0, Start: 0, End: 20, Title: "Opening Credits"}}
		},
		Markers: func(_ library.Item, markers []Marker) []Marker {
			return append(markers, Marker{Type: "credits", Label: "Credits", Start: 100, End: 120, Source: "visual"})
		},
	}
	for range 2 {
		result := probe.Inspect(t.Context(), item, enrichment)
		if len(result.Markers) != 2 || len(result.RandomAccess) != 1 || result.RandomAccess[0] != 10.5 {
			t.Fatalf("enriched result = %#v", result)
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("random access calls = %q, error = %v", data, err)
	}
}

func TestProbeConfigurationDurationAndResultValidation(t *testing.T) {
	t.Parallel()
	probe := New("unused")
	probe.ConfigureCache(t.TempDir())
	if probe.cachePath("film") == "" {
		t.Fatal("configured cache path is empty")
	}
	if New("unused").cachePath("film") != "" {
		t.Fatal("unconfigured cache path is not empty")
	}
	invalidResults := []Result{
		{Duration: math.NaN()},
		{Duration: -1},
		{Duration: 366*24*60*60 + 1},
		{Bitrate: -1},
		{Video: VideoFacts{Width: -1}},
		{Chapters: []Chapter{{Start: 1, End: 1}}},
		{Markers: []Marker{{Start: 1, End: math.Inf(1)}}},
		{RandomAccess: []float64{-1}},
		{Audio: make([]AudioTrack, 65)},
	}
	for _, result := range invalidResults {
		if validProbeResult(result) {
			t.Fatalf("invalid result was accepted: %#v", result)
		}
	}
	probe.cache["film"] = Result{Duration: 42}
	if duration := probe.Duration(t.Context(), library.Item{ID: "film", Path: "missing"}); duration != 42 {
		t.Fatalf("cached duration = %v", duration)
	}
}

func TestProbeDurationRunsAndLoadsBoundedResults(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	media := filepath.Join(root, "film.mp4")
	executable := filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"73\"}}'\n")
	item := library.Item{ID: "film", Kind: "video", Path: media}
	first := New(executable)
	first.ConfigureCache(cache)
	if duration := first.Inspect(t.Context(), item, Enrichment{}).Duration; duration != 73 {
		t.Fatalf("first duration = %v", duration)
	}
	second := New("missing")
	second.ConfigureCache(cache)
	if duration := second.Duration(t.Context(), item); duration != 73 {
		t.Fatalf("loaded duration = %v", duration)
	}
	if duration := New(filepath.Join(root, "missing")).Duration(t.Context(), item); duration != 0 {
		t.Fatalf("failed duration = %v", duration)
	}
}

func TestProbeCommandBoundsOutputAndExecutable(t *testing.T) {
	t.Parallel()
	output := boundedOutput{remaining: 2}
	if written, err := output.Write([]byte("abc")); written != 2 || err == nil || output.String() != "ab" {
		t.Fatalf("bounded write = %d, %v, %q", written, err, output.String())
	}
	if _, err := runProbe(t.Context(), "", "x"); err == nil {
		t.Fatal("empty executable was accepted")
	}
	if _, err := runProbe(t.Context(), strings.Repeat("x", 4097)); err == nil {
		t.Fatal("oversized executable was accepted")
	}
	if _, err := runProbe(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing executable succeeded")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	executable := filepath.Join(t.TempDir(), "slow")
	writeProbeScript(t, executable, "#!/bin/sh\nsleep 1\n")
	if _, err := runProbe(ctx, executable); err == nil {
		t.Fatal("canceled probe succeeded")
	}
}

func TestProbeChapterMarkerAndTagNormalization(t *testing.T) { //nolint:cyclop // One fixture covers related probe normalization rules.
	t.Parallel()
	data := `{"streams":[{"codec_type":"audio","codec_name":"aac"}],"chapters":[` +
		`{"start_time":"0","end_time":"10","tags":{"title":"Previously on"}},` +
		`{"start_time":"10","end_time":"20","tags":{"title":"Opening title"}},` +
		`{"start_time":"20","end_time":"30","tags":{"title":"Ad break"}},` +
		`{"start_time":"30","end_time":"40","tags":{"title":"Epilogue"}},` +
		`{"start_time":"40","end_time":"50","tags":{"title":"Credit roll"}},` +
		`{"start_time":"bad","end_time":"60"},{"start_time":"60","end_time":"60"}],` +
		`"format":{"tags":{"TRACK":"2/9","disc":"bad"}}}`
	result, valid := parse([]byte(data))
	if !valid || len(result.Markers) != 4 || len(result.Chapters) != 5 || result.Audio[0].Label != "Audio 1 · AAC" || result.Tags.Track != 2 || result.Tags.Disc != 0 {
		t.Fatalf("normalized result = %#v", result)
	}
	if (Chapter{Start: 65}).Timestamp() != "1:05" || (Chapter{Start: 3661}).Timestamp() != "1:01:01" {
		t.Fatal("chapter timestamp is invalid")
	}
	if sameChapterTitles([]Chapter{{Title: "A"}}, []Chapter{{Title: "B"}}) || sameChapterTitles(nil, []Chapter{{}}) || !sameChapterTitles([]Chapter{{Title: "A"}}, []Chapter{{Title: "A"}}) {
		t.Fatal("chapter title comparison is invalid")
	}
}
