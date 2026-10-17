package markers

import (
	"context"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestVisualCreditsParsingAndAnalysisBoundaries(t *testing.T) {
	t.Parallel()
	analyzer := &Analyzer{ffmpeg: "/path/that/does/not/exist"}
	if markers, err := analyzer.visualCredits(t.Context(), library.Item{}, 50); err != nil || markers != nil {
		t.Fatalf("short media = %#v, %v", markers, err)
	}
	if _, err := analyzer.visualCredits(t.Context(), library.Item{}, 200); err == nil {
		t.Fatal("missing analyzer executable succeeded")
	}
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	if markers, err := analyzer.visualCredits(context.Background(), library.Item{}, 200); err != nil || markers != nil {
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

func TestCreditRollMotionPromotesTerminalCreditsOnly(t *testing.T) {
	t.Parallel()
	roll := creditFrameProfile(creditFixture(12, 245))
	shifted := creditFrameProfile(shiftedCreditFixture(12, 245, 9))
	if !creditTextMoves(roll.rows, shifted.rows) {
		t.Fatal("scrolling credit roll was not recognized as moving text")
	}
	if creditTextMoves(roll.rows, roll.rows) {
		t.Fatal("identical static frames were recognized as moving text")
	}
	card := creditFrameProfile(cardCreditFixture(12, 245))
	if !creditTextMoves(roll.rows, card.rows) {
		t.Fatal("card change was not recognized as moving text")
	}
	terminal := creditMarkers(creditPoints(0, 60, true), 940, 1000)
	if len(terminal) != 1 || terminal[0].Source != "text" || terminal[0].End != 1000 || !validPersistedMarker(terminal[0]) {
		t.Fatalf("rolling terminal credits = %#v", terminal)
	}
	static := creditMarkers(creditPoints(0, 60, false), 940, 1000)
	if len(static) != 1 || static[0].Source != "visual" {
		t.Fatalf("static terminal credits = %#v", static)
	}
	short := creditMarkers(creditPoints(0, 25, true), 975, 1000)
	if len(short) != 1 || short[0].Source != "visual" {
		t.Fatalf("short terminal credits = %#v", short)
	}
	split := creditMarkers(append(creditPoints(0, 40, true), creditPoints(60, 40, true)...), 900, 1000)
	if len(split) != 2 || split[0].Source != "visual" || split[1].Source != "text" {
		t.Fatalf("mid-credit split = %#v", split)
	}
}

func TestVisualCreditsPublishTextTierFromScrollingFrames(t *testing.T) {
	t.Parallel()
	analyzer := &Analyzer{}
	analyzer.extractCredits = func(_ context.Context, _ library.Item, _, length float64) ([]byte, error) {
		frames := make([]byte, 0, int(length)*creditFrameWidth*creditFrameHeight)
		for second := range int(length) {
			if second < 20 {
				frames = append(frames, make([]byte, creditFrameWidth*creditFrameHeight)...)
				continue
			}
			frames = append(frames, shiftedCreditFixture(12, 245, second*9%63)...)
		}
		return frames, nil
	}
	markers, err := analyzer.visualCredits(t.Context(), library.Item{}, 300)
	if err != nil || len(markers) != 1 || markers[0].Source != "text" || markers[0].Start != 260 || markers[0].End != 300 {
		t.Fatalf("scrolling credits = %#v, %v", markers, err)
	}
	analyzer.extractCredits = func(_ context.Context, _ library.Item, _, length float64) ([]byte, error) {
		frames := make([]byte, 0, int(length)*creditFrameWidth*creditFrameHeight)
		for range int(length) {
			frames = append(frames, creditFixture(12, 245)...)
		}
		return frames, nil
	}
	markers, err = analyzer.visualCredits(t.Context(), library.Item{}, 300)
	if err != nil || len(markers) != 1 || markers[0].Source != "visual" {
		t.Fatalf("static text-like tail = %#v, %v", markers, err)
	}
}

func shiftedCreditFixture(background, foreground byte, shift int) []byte {
	source := creditFixture(background, foreground)
	frame := make([]byte, len(source))
	for index := range frame {
		frame[index] = background
	}
	copy(frame, source[shift*creditFrameWidth:])
	return frame
}

func cardCreditFixture(background, foreground byte) []byte {
	frame := make([]byte, creditFrameWidth*creditFrameHeight)
	for index := range frame {
		frame[index] = background
	}
	for row := 13; row < 80; row += 11 {
		for column := 20; column < 140; column += 10 {
			for y := row; y < row+2; y++ {
				for x := column; x < column+6; x++ {
					frame[y*creditFrameWidth+x] = foreground
				}
			}
		}
	}
	return frame
}
