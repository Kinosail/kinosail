package markers

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestVisualFingerprintCacheAndMalformedOutput(t *testing.T) { //nolint:cyclop // The assertions cover one fingerprint cache workflow.
	t.Parallel()
	directory := t.TempDir()
	tool := filepath.Join(directory, "visual-tool")
	arguments := filepath.Join(directory, "arguments")
	fixture := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\ncase \"$*\" in *scale=160:90*) head -c 14400 /dev/zero;; *) head -c 144 /dev/zero;; esac\n", arguments)
	if err := os.WriteFile(tool, []byte(fixture), 0o700); err != nil { //nolint:gosec // This private fixture must be executable.
		t.Fatal(err)
	}
	item := library.Item{ID: "item", Path: filepath.Join(directory, "media.mp4")}
	analyzer := &Analyzer{cache: filepath.Join(directory, "cache"), ffmpeg: tool}
	points, err := analyzer.visualFingerprint(t.Context(), item, "opening", 0, 10)
	if err != nil || !reflect.DeepEqual(points, []uint64{0, 0}) {
		t.Fatalf("visual fingerprint = %#v, %v", points, err)
	}
	if frames, frameErr := analyzer.creditFrames(t.Context(), item, 0, 10); frameErr != nil || len(frames) != 14400 {
		t.Fatalf("credit frames = %d bytes, %v", len(frames), frameErr)
	}
	logged, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		if !strings.Contains(command, "-threads 1") || !strings.Contains(command, "-filter_threads 1") {
			t.Fatalf("background analysis can consume all CPU: %s", command)
		}
	}
	analyzer.ffmpeg = filepath.Join(directory, "missing")
	if cached, cacheErr := analyzer.visualFingerprint(t.Context(), item, "opening", 0, 10); cacheErr != nil || !reflect.DeepEqual(cached, points) {
		t.Fatalf("cached visual fingerprint = %#v, %v", cached, cacheErr)
	}
	bad := filepath.Join(directory, "bad-visual-tool")
	if err = os.WriteFile(bad, []byte("#!/bin/sh\nhead -c 73 /dev/zero\n"), 0o700); err != nil { //nolint:gosec // This private fixture must be executable.
		t.Fatal(err)
	}
	analyzer.ffmpeg = bad
	if _, err = analyzer.visualFingerprint(t.Context(), item, "tail", 0, 10); err == nil {
		t.Fatal("malformed visual fingerprint was accepted")
	}
	if _, err = (&Analyzer{}).visualFingerprint(t.Context(), item, "opening", 0, 10); err == nil {
		t.Fatal("missing visual cache configuration was accepted")
	}
}

func TestMalformedCreditFramesPreserveLastGoodMarkers(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "movie", Kind: "video", Size: 1, Added: time.Unix(1, 0)}
	marker := Marker{Type: "credits", Label: "Credits", Start: 7000, End: 7200, Source: "visual"}
	analyzer := NewAnalyzer(Config{})
	analyzer.records[item.ID] = Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion, Markers: []Marker{marker}}
	analyzer.extractCredits = func(_ context.Context, _ library.Item, _, _ float64) ([]byte, error) { return []byte{1}, nil }
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 7200} })
	if err := analyzer.analyzeBatch(t.Context(), []library.Item{item}); err == nil {
		t.Fatal("malformed credits succeeded")
	}
	if got := analyzer.Markers(item, nil); !reflect.DeepEqual(got, []Marker{marker}) {
		t.Fatalf("markers after malformed frames = %#v", got)
	}
}

func TestMarkerExtractorsRejectInvalidWindowsWithoutSideEffects(t *testing.T) {
	t.Parallel()
	calls := 0
	analyzer := &Analyzer{
		extract: func(context.Context, library.Item, string, float64, float64) ([]uint32, error) {
			calls++
			return nil, nil
		},
		extractVisual: func(context.Context, library.Item, string, float64, float64) ([]uint64, error) {
			calls++
			return nil, nil
		},
		extractCredits: func(context.Context, library.Item, float64, float64) ([]byte, error) {
			calls++
			return nil, nil
		},
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 0, 901} {
		if _, err := analyzer.fingerprint(t.Context(), library.Item{}, "opening", 0, value); err == nil {
			t.Fatalf("audio length %v was accepted", value)
		}
		if _, err := analyzer.visualFingerprint(t.Context(), library.Item{}, "opening", 0, value); err == nil {
			t.Fatalf("visual length %v was accepted", value)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 0} {
		if _, err := analyzer.visualCredits(t.Context(), library.Item{}, value); err == nil {
			t.Fatalf("credit duration %v was accepted", value)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid windows caused %d extractor calls", calls)
	}
}
