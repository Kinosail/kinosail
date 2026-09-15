package markers

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSharedFingerprintSegmentFindsShiftedRecurringAudio(t *testing.T) {
	shared := make([]uint32, 200)
	for index := range shared {
		shared[index] = uint32(index*7919 + 17)
	}
	left := append(append(sequence(31, 1), shared...), sequence(40, 9000)...)
	right := append(append(sequence(67, 4000), shared...), sequence(20, 12000)...)
	gotLeft, gotRight, ok := sharedFingerprintSegment(left, right)
	if !ok || gotLeft.Start < 3.7 || gotLeft.Start > 4 || gotRight.Start < 8.2 || gotRight.Start > 8.4 || gotLeft.End-gotLeft.Start < 24 {
		t.Fatalf("segments = %#v %#v, %v", gotLeft, gotRight, ok)
	}
}

func TestSharedVisualSegmentFindsReencodedRecurringOpening(t *testing.T) {
	shared := visualSequence(45, 1000)
	reencoded := make([]uint64, len(shared))
	for index, value := range shared {
		reencoded[index] = value ^ 1
	}
	left := append(append(visualSequence(11, 1), shared...), visualSequence(20, 9000)...)
	right := append(append(visualSequence(27, 4000), reencoded...), visualSequence(10, 12000)...)
	gotLeft, gotRight, ok := sharedVisualSegment(left, right)
	if !ok || gotLeft.Start != 11 || gotRight.Start != 27 || gotLeft.End-gotLeft.Start < 40 {
		t.Fatalf("segments = %#v %#v, %v", gotLeft, gotRight, ok)
	}
}

func TestMarkerAnalyzerRequiresSeasonConsensusAndPersistsResults(t *testing.T) {
	sharedOpening, sharedTail := sequence(200, 1000), sequence(180, 9000)
	items := []library.Item{
		{ID: "one", Kind: "video", Show: "Show", Season: 1, Episode: 1, Size: 1, Added: time.Unix(1, 0)},
		{ID: "two", Kind: "video", Show: "Show", Season: 1, Episode: 2, Size: 2, Added: time.Unix(2, 0)},
		{ID: "three", Kind: "video", Show: "Show", Season: 1, Episode: 3, Size: 3, Added: time.Unix(3, 0)},
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir(), CacheDir: t.TempDir(), FFmpeg: "ffmpeg", Fingerprint: "fpcalc"})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 1800} })
	analyzer.extract = func(_ context.Context, item library.Item, window string, _, _ float64) ([]uint32, error) {
		prefix := sequence(item.Episode*13, uint32(item.Episode)*100000) //nolint:gosec // Fixture episode numbers are bounded.
		if window == "tail" {
			return append(sequence(2700+item.Episode*13, uint32(item.Episode)*100000), sharedTail...), nil //nolint:gosec // Fixture episode numbers are bounded.
		}
		return append(prefix, sharedOpening...), nil
	}
	if err := analyzer.analyzeSeason(t.Context(), items); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		markers := analyzer.Markers(item, nil)
		if len(markers) != 2 || markers[0].Type != "intro" || markers[1].Type != "outro" {
			t.Fatalf("%s markers = %#v", item.ID, markers)
		}
	}
	reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)})
	if reloaded.err != nil {
		t.Fatal(reloaded.err)
	}
	if got := reloaded.Markers(items[0], nil); len(got) != 2 {
		t.Fatalf("persisted markers = %#v", got)
	}
}

func TestMarkerAnalyzerUsesVisualConsensusForSilentShowIntro(t *testing.T) {
	sharedOpening := visualSequence(45, 9000)
	items := []library.Item{
		{ID: "one", Kind: "video", Show: "Silent Show", Season: 1, Episode: 1, Size: 1, Added: time.Unix(1, 0)},
		{ID: "two", Kind: "video", Show: "Silent Show", Season: 1, Episode: 2, Size: 2, Added: time.Unix(2, 0)},
		{ID: "three", Kind: "video", Show: "Silent Show", Season: 1, Episode: 3, Size: 3, Added: time.Unix(3, 0)},
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir(), CacheDir: t.TempDir()})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 1800} })
	analyzer.extract = func(_ context.Context, item library.Item, _ string, _, _ float64) ([]uint32, error) {
		return sequence(300, uint32(item.Episode)*100000), nil //nolint:gosec // Fixture episodes are bounded.
	}
	analyzer.extractVisual = func(_ context.Context, item library.Item, window string, _, _ float64) ([]uint64, error) {
		if window == "tail" {
			return visualSequence(300, uint64(item.Episode)*100000), nil //nolint:gosec // Fixture episodes are bounded.
		}
		return append(visualSequence(10+item.Episode, uint64(item.Episode)*100000), sharedOpening...), nil //nolint:gosec // Fixture episodes are bounded.
	}
	if err := analyzer.analyzeSeason(t.Context(), items); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		markers := analyzer.Markers(item, nil)
		if len(markers) != 1 || markers[0].Type != "intro" || markers[0].Source != "recurrence" {
			t.Fatalf("%s markers = %#v", item.ID, markers)
		}
	}
}

func TestMatchingAndConsensusBoundaries(t *testing.T) {
	if _, _, ok := sharedFingerprintSegment(sequence(100, 500), sequence(100, 500)); ok {
		t.Fatal("short repeated audio was accepted")
	}
	if _, _, ok := sharedFingerprintSegment(sequence(300, 1), sequence(300, 10000)); ok {
		t.Fatal("unrelated audio was accepted")
	}
	candidate := markerCandidate{}
	acceptCandidate(&candidate, timeRange{10, 40}, 400, false)
	acceptCandidate(&candidate, timeRange{100, 130}, 400, false)
	if _, ok := candidate.consensus(2); ok {
		t.Fatalf("candidate = %#v", candidate)
	}
	candidate = markerCandidate{}
	acceptCandidate(&candidate, timeRange{8, 72}, 400, false)
	acceptCandidate(&candidate, timeRange{10, 70}, 400, false)
	acceptCandidate(&candidate, timeRange{12, 68}, 400, false)
	if got, ok := candidate.consensus(3); !ok || got != (timeRange{12, 68}) {
		t.Fatalf("consensus = %#v, %v", got, ok)
	}
}

func TestManualMarkerOverridesDetectedMarkersAndRejectsInvalidInput(t *testing.T) {
	item := library.Item{ID: "episode", Size: 12, Added: time.Unix(3, 0)}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
	if err := analyzer.SetManual(item, "intro", 5, 55, 100); err != nil {
		t.Fatal(err)
	}
	if err := analyzer.SetManual(item, "intro", 6, 56, 100); err != nil {
		t.Fatal(err)
	}
	analyzer.replaceDetected(item, []string{"intro"}, []Marker{{Type: "intro", Start: 12, End: 60, Source: "fingerprint"}})
	got := analyzer.Markers(item, []Marker{{Type: "intro", Start: 10, End: 70, Source: "chapter"}})
	if len(got) != 1 || got[0].Source != "manual" || got[0].Start != 6 || got[0].End != 56 {
		t.Fatalf("markers = %#v", got)
	}
	invalid := []struct {
		markerType string
		start      float64
		end        float64
	}{{start: 1, end: 2}, {markerType: "unknown", start: 1, end: 2}, {markerType: "intro", start: -1, end: 2}, {markerType: "intro", start: 60, end: 50}, {markerType: "intro", start: 60, end: 101}, {markerType: "intro", start: math.NaN(), end: 70}}
	for _, input := range invalid {
		if err := analyzer.SetManual(item, input.markerType, input.start, input.end, 100); err == nil {
			t.Fatalf("invalid manual marker was accepted: %#v", input)
		}
		if unchanged := analyzer.Markers(item, nil); !reflect.DeepEqual(unchanged, got) {
			t.Fatalf("invalid marker changed state to %#v", unchanged)
		}
	}
}

func TestOwnerRemovalSuppressesDetectedMarkerAcrossRestart(t *testing.T) {
	item := library.Item{ID: "movie", Size: 12, Added: time.Unix(3, 0)}
	dataDir := t.TempDir()
	analyzer := NewAnalyzer(Config{DataDir: dataDir})
	detected := []Marker{{Type: "credits", Label: "Credits", Start: 80, End: 100, Source: "visual"}}
	analyzer.replaceDetected(item, []string{"credits"}, detected)
	if err := analyzer.Suppress(item, "credits"); err != nil {
		t.Fatal(err)
	}
	if err := analyzer.Suppress(item, "unknown"); err == nil {
		t.Fatal("unknown marker type was suppressed")
	}
	analyzer.replaceDetected(item, []string{"credits"}, detected)
	if got := analyzer.Markers(item, []Marker{{Type: "credits", Start: 75, End: 100, Source: "chapter"}}); len(got) != 0 {
		t.Fatalf("removed credits = %#v", got)
	}
	reloaded := NewAnalyzer(Config{DataDir: dataDir})
	if got := reloaded.Markers(item, detected); len(got) != 0 {
		t.Fatalf("reloaded credits = %#v", got)
	}
}

func TestInvalidPersistedSuppressionDoesNotHideMarkers(t *testing.T) {
	item := library.Item{ID: "movie", Size: 12, Added: time.Unix(3, 0)}
	detected := []Marker{{Type: "credits", Start: 80, End: 100, Source: "visual"}}
	record := Record{Revision: mediaRevision(item), DetectorVersion: DetectorVersion, Markers: detected, Suppressed: []string{"credits", "unknown"}}
	analyzer := NewAnalyzer(Config{})
	analyzer.records[item.ID] = record
	if got := analyzer.Markers(item, nil); !reflect.DeepEqual(got, detected) || !reflect.DeepEqual(analyzer.records[item.ID], record) {
		t.Fatalf("invalid suppression changed markers or state: %#v %#v", got, analyzer.records[item.ID])
	}
}

func TestAnalyzerRejectsInvalidPersistedMarkerState(t *testing.T) {
	t.Parallel()
	for name, state := range map[string]string{
		"unknown field": `{"movie":{"revision":"1:1","markers":[],"extra":true}}`,
		"invalid range": `{"movie":{"revision":"1:1","markers":[{"type":"intro","label":"Intro","start":10,"end":1,"source":"manual"}]}}`,
		"unsafe ID":     `{"../movie":{"revision":"1:1","markers":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "playback_markers.json"), []byte(state), 0o600); err != nil {
				t.Fatal(err)
			}
			analyzer := NewAnalyzer(Config{DataDir: directory})
			_, items, message := analyzer.Status()
			if items != 0 || message == "" {
				t.Fatalf("invalid state loaded: items=%d error=%q", items, message)
			}
		})
	}
}

func TestCreditMarkersKeepMidCreditSceneWatchable(t *testing.T) {
	points := append(floatSequence(0, 31), floatSequence(45, 26)...)
	want := []Marker{{Type: "credits", Label: "Credits", Start: 900, End: 931, Source: "visual"}, {Type: "credits", Label: "Credits", Start: 945, End: 971, Source: "visual"}}
	if got := creditMarkers(points, 900, 1000); !reflect.DeepEqual(got, want) {
		t.Fatalf("credits = %#v, want %#v", got, want)
	}
	if got := creditMarkers(floatSequence(0, 31), 900, 1100); len(got) != 0 {
		t.Fatalf("unanchored credits = %#v", got)
	}
}

func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' {
			return path[:index]
		}
	}
	return "."
}

func sequence(length int, offset uint32) []uint32 {
	values := make([]uint32, length)
	for index := range values {
		values[index] = offset + uint32(index*104729)
	}
	return values
}

func visualSequence(length int, offset uint64) []uint64 {
	values, value := make([]uint64, length), offset|1
	for index := range values {
		value ^= value << 13
		value ^= value >> 7
		value ^= value << 17
		values[index] = value
	}
	return values
}

func floatSequence(start, length int) []float64 {
	values := make([]float64, length)
	for index := range values {
		values[index] = float64(start + index)
	}
	return values
}
