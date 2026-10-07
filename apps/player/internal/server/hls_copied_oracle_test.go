package server_test

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// These controls protect the decoded-media E2E oracle, not an app endpoint.
// A decoder may emit its explicitly discarded source primer. No absent padding,
// partial frame, inconsistent clock or missing audible output may be excused.
func TestCopiedAudioOracleRejectsUnprovenSourcePriming(t *testing.T) {
	for name, change := range map[string]func(*copiedPrimerControl){
		"absent padding":        func(c *copiedPrimerControl) { c.padding = 0 },
		"mismatched padding":    func(c *copiedPrimerControl) { c.padding = 512 },
		"missing sample rate":   func(c *copiedPrimerControl) { c.rate = 0 },
		"wrong sample rate":     func(c *copiedPrimerControl) { c.rate = 48000 },
		"partial decoded frame": func(c *copiedPrimerControl) { c.frames[0].samples = 512 },
		"wrong negative clock":  func(c *copiedPrimerControl) { c.frames[0].time = -0.01 },
		"nonfinite clock":       func(c *copiedPrimerControl) { c.frames[0].time = math.NaN() },
		"negative next frame":   func(c *copiedPrimerControl) { c.frames[1].time = -0.01 },
		"late audible frame":    func(c *copiedPrimerControl) { c.frames[1].time = 0.01 },
		"no audible frame":      func(c *copiedPrimerControl) { c.frames = c.frames[:1] },
		"no decoded frames":     func(c *copiedPrimerControl) { c.frames = nil },
	} {
		t.Run(name, func(t *testing.T) {
			control := newCopiedPrimerControl()
			change(&control)
			if _, err := audibleCopiedDecodedSource(control.frames, control.padding, control.rate); err == nil {
				t.Fatal("unproven source-frame exclusion was accepted")
			}
		})
	}
}

func TestCopiedAudioOraclePreservesAudibleSourceFrames(t *testing.T) {
	control := newCopiedPrimerControl()
	before := append([]copiedDecodedFrame(nil), control.frames...)
	actual, err := audibleCopiedDecodedSource(control.frames, control.padding, control.rate)
	if err != nil || !reflect.DeepEqual(actual, before[1:]) || !reflect.DeepEqual(control.frames, before) {
		t.Fatal("measured full source primer was not excluded without changing audible frames")
	}
	control.frames = before[1:]
	actual, err = audibleCopiedDecodedSource(control.frames, control.padding, control.rate)
	if err != nil || !reflect.DeepEqual(actual, before[1:]) {
		t.Fatal("decoder that already discarded its primer lost an audible frame")
	}
}

func TestCopiedAudioOracleRejectsLostOrDuplicatedDeliveredFrames(t *testing.T) {
	if mode := os.Getenv("KINOSAIL_COPIED_ORACLE_CONTROL"); mode != "" {
		copiedAudioFrameCountControl(t, mode)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"lost", "duplicated"} {
		t.Run(mode, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), binary, "-test.run=^TestCopiedAudioOracleRejectsLostOrDuplicatedDeliveredFrames$") //nolint:gosec // Current owned test executable and fixed arguments.
			command.Env = []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR"), "KINOSAIL_COPIED_ORACLE_CONTROL=" + mode}
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "audio decoded count") {
				t.Fatalf("%s delivered-frame control did not fail its strict count oracle: %v", mode, err)
			}
		})
	}
}

type copiedPrimerControl struct {
	frames  []copiedDecodedFrame
	padding int
	rate    int
}

func newCopiedPrimerControl() copiedPrimerControl {
	return copiedPrimerControl{
		frames: []copiedDecodedFrame{
			{time: -0.022993, samples: 1024, hash: strings.Repeat("0", 64)},
			{time: 0.000227, samples: 1024, hash: strings.Repeat("1", 64)},
			{time: 0.023447, samples: 1024, hash: strings.Repeat("2", 64)},
		},
		padding: 1024,
		rate:    44100,
	}
}

func copiedAudioFrameCountControl(t *testing.T, mode string) {
	t.Helper()
	control := newCopiedPrimerControl()
	expected, err := audibleCopiedDecodedSource(control.frames, control.padding, control.rate)
	if err != nil {
		t.Fatal(err)
	}
	var actual []copiedDecodedFrame
	switch mode {
	case "lost":
		actual = control.frames[1:2]
	case "duplicated":
		actual = append(append([]copiedDecodedFrame(nil), control.frames[1:]...), control.frames[1])
	default:
		t.Fatal("unknown owned oracle control")
	}
	assertCopiedFrameCount(t, "audio", expected, actual)
}

// This source-only exclusion is independent of the delivered frame count.
// The public journey first validates the declared full negative packet against
// actual FFprobe metadata. FFmpeg6 can decode it despite its CodecDelay.
func audibleCopiedDecodedSource(frames []copiedDecodedFrame, padding, rate int) ([]copiedDecodedFrame, error) {
	if len(frames) == 0 {
		return nil, errors.New("missing decoded source frames")
	}
	if frames[0].time >= 0 {
		return frames, nil // FFmpeg9 already discards the primer.
	}
	if len(frames) < 2 || padding != 1024 || rate != 44100 || frames[0].samples != padding {
		return nil, errors.New("decoded source primer lacks its full declared padding")
	}
	if !copiedDecodedPrimerClock(frames[0], frames[1], padding, rate) {
		return nil, errors.New("decoded source primer conflicts with its audible boundary")
	}
	return frames[1:], nil
}

func copiedDecodedPrimerClock(first, next copiedDecodedFrame, padding, rate int) bool {
	return !math.IsNaN(first.time) && !math.IsInf(first.time, 0) &&
		math.Abs(first.time+float64(padding)/float64(rate)) <= 0.001001 &&
		next.time >= 0 && next.time <= 0.001001
}
