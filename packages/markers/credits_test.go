package markers

import (
	"context"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestVisualCreditsParsingAndAnalysisBoundaries(t *testing.T) { //nolint:cyclop // Parser, duration floor, tool error, and successful execution are one analyzer boundary.
	t.Parallel()
	analyzer := &Analyzer{ffmpeg: "/path/that/does/not/exist"}
	if markers, err := analyzer.visualCredits(t.Context(), library.Item{}, 50); err != nil || markers != nil {
		t.Fatalf("short media = %#v, %v", markers, err)
	}
	if _, err := analyzer.visualCredits(t.Context(), library.Item{}, 200); err == nil {
		t.Fatal("missing analyzer executable succeeded")
	}
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	if markers, err := analyzer.visualCredits(context.Background(), library.Item{}, 200); err != nil || markers != nil { //nolint:usetesting // Explicit background context proves the analyzer owns its timeout.
		t.Fatalf("successful empty analysis = %#v, %v", markers, err)
	}
}

func TestCreditFrameClassifierRequiresSustainedTextLikeStructure(t *testing.T) {
	t.Parallel()
	for name, frame := range map[string][]byte{
		"dark credits":   creditFixture(12, 245),
		"bright credits": creditFixture(240, 15),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !creditFrameLikely(frame) {
				t.Fatal("text-like credit frame was rejected")
			}
		})
	}
	if creditFrameLikely(make([]byte, 160*90)) {
		t.Fatal("uniform dark story frame was accepted")
	}
	if creditFrameLikely([]byte{1, 2, 3}) {
		t.Fatal("malformed frame was accepted")
	}
}

func creditFixture(background, foreground byte) []byte {
	frame := make([]byte, 160*90)
	for index := range frame {
		frame[index] = background
	}
	for row := 10; row < 80; row += 7 {
		for column := 25; column < 135; column += 14 {
			for y := row; y < row+3; y++ {
				for x := column; x < column+9; x++ {
					frame[y*160+x] = foreground
				}
			}
		}
	}
	return frame
}
