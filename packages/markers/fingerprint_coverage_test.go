package markers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFingerprintTemporaryFileCreationFailure(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	blocked := filepath.Join(directory, "blocked")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	analyzer := &Analyzer{cache: blocked, ffmpeg: "ffmpeg", tool: "fpcalc"}
	item := library.Item{ID: "item", Path: filepath.Join(directory, "media.mp4")}
	if _, err := analyzer.fingerprint(t.Context(), item, "tail", 1, 10); err == nil {
		t.Fatal("temporary audio file was created in an unwritable directory")
	}
}

func TestVisualFingerprintSetsComparisonBits(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	tool := filepath.Join(directory, "visual-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf '\\377\\000'\nhead -c 70 /dev/zero\n"), 0o700); err != nil { //nolint:gosec // Test-owned executable fixture.
		t.Fatal(err)
	}
	analyzer := &Analyzer{cache: filepath.Join(directory, "cache"), ffmpeg: tool}
	points, err := analyzer.visualFingerprint(t.Context(), library.Item{ID: "item", Path: "media.mp4"}, "opening", 0, 10)
	if err != nil || len(points) != 1 || points[0] == 0 {
		t.Fatalf("visual fingerprint = %#v, %v", points, err)
	}
}

func TestVisualCreditsCollectsLikelyFramesAndClosesAtDuration(t *testing.T) {
	t.Parallel()
	analyzer := NewAnalyzer(Config{})
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) {
		return creditFixture(12, 245), nil
	}
	if markers, err := analyzer.visualCredits(t.Context(), library.Item{}, 200); err != nil || markers != nil {
		t.Fatalf("single likely frame = %#v, %v", markers, err)
	}
	markers := creditMarkers(floatSequence(0, 21), 79, 100)
	if len(markers) != 1 || markers[0].End != 100 {
		t.Fatalf("closing credit marker = %#v", markers)
	}
}
