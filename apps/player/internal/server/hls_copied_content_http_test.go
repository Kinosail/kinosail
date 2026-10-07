package server_test

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type copiedDecodedFrame struct {
	time    float64
	hash    string
	samples int
}

func copiedDecodedFrames(t *testing.T, ctx context.Context, ffmpeg, path, stream string) []copiedDecodedFrame {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-err_detect", "explode", "-copyts", "-i", path, "-map", stream, "-enc_time_base", "1:1000000", "-fps_mode", "passthrough", "-f", "framehash", "-hash", "sha256", "-") //nolint:gosec // Discovered local tool and owned generated media.
	data, err := command.Output()
	if err != nil {
		t.Fatalf("decode %s sequence: %v", stream, err)
	}
	var frames []copiedDecodedFrame
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		frames = append(frames, parseCopiedDecodedFrame(t, line, stream))
	}
	if len(frames) == 0 {
		t.Fatal("empty decoded sequence")
	}
	return frames
}

func parseCopiedDecodedFrame(t *testing.T, line, stream string) copiedDecodedFrame {
	t.Helper()
	fields := strings.Split(line, ",")
	if len(fields) != 6 {
		t.Fatal("malformed decoded frame metadata")
	}
	pts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSpace(fields[5])
	if len(hash) != 64 {
		t.Fatal("decoded frame hash missing")
	}
	size, err := strconv.Atoi(strings.TrimSpace(fields[4]))
	if err != nil || size <= 0 {
		t.Fatal("decoded frame size missing")
	}
	samples := 0
	if stream == "0:a:0" {
		if size%2 != 0 {
			t.Fatal("mono PCM sample size invalid")
		}
		samples = size / 2
	}
	return copiedDecodedFrame{time: float64(pts) / 1000000, hash: hash, samples: samples}
}

// Full-source decoding is an independent oracle for public segment continuity.
// Only a single initial mux clock is normalized, preserving all subsequent PTS.
func assertCopiedHLSContent(t *testing.T, ctx context.Context, ffmpeg, source string, media []byte, audio bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "delivered.mp4")
	if err := os.WriteFile(path, media, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := copiedDecodedFrames(t, ctx, ffmpeg, source, "0:v:0")
	actual := copiedDecodedFrames(t, ctx, ffmpeg, path, "0:v:0")
	assertCopiedFrameCount(t, "video", expected, actual)
	if len(actual) != 144 {
		t.Fatalf("12-second/12fps fixture decoded %d video frames", len(actual))
	}
	clock := actual[0].time - expected[0].time
	for index := range expected {
		if actual[index].hash != expected[index].hash {
			t.Fatalf("video frame %d differs from full source", index)
		}
		assertCopiedFrameClock(t, "0:v:0", index, expected[index].time, actual[index].time, clock)
	}
	t.Logf("0:v:0: %d decoded frames; shared initial mux shift %.6f seconds", len(actual), clock)
	if audio {
		assertCopiedAudioPackets(t, ctx, source, path, clock)
		assertCopiedAudioDecode(t, ctx, ffmpeg, source, path, clock)
	}
}

func assertCopiedFrameCount(t *testing.T, stream string, expected, actual []copiedDecodedFrame) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s decoded count = %d, source = %d", stream, len(actual), len(expected))
	}
}

func assertCopiedFrameClock(t *testing.T, stream string, index int, source, delivered, clock float64) {
	t.Helper()
	// MKV source timestamps use 1ms ticks; MP4 preserves finer video/audio ticks.
	if math.Abs(delivered-source-clock) > 0.001001 {
		t.Fatalf("%s frame %d timestamp drift: source %.6f, delivered %.6f, mux %.6f", stream, index, source, delivered, clock)
	}
}

func assertCopiedAudioDecode(t *testing.T, ctx context.Context, ffmpeg, source, path string, clock float64) {
	t.Helper()
	expected := copiedDecodedFrames(t, ctx, ffmpeg, source, "0:a:0")
	actual := copiedDecodedFrames(t, ctx, ffmpeg, path, "0:a:0")
	t.Logf("AAC decoded source frames=%d samples=%d firstPTS=%.6f firstSamples=%d; delivered frames=%d samples=%d firstPTS=%.6f firstSamples=%d", len(expected), copiedDecodedSampleTotal(expected), expected[0].time, expected[0].samples, len(actual), copiedDecodedSampleTotal(actual), actual[0].time, actual[0].samples)
	assertCopiedFrameCount(t, "audio", expected, actual)
	samplesExpected, samplesActual := 0, 0
	var differing []int
	for index := range expected {
		samplesExpected += expected[index].samples
		samplesActual += actual[index].samples
		if actual[index].hash != expected[index].hash {
			differing = append(differing, index)
		}
		assertCopiedFrameClock(t, "0:a:0", index, expected[index].time, actual[index].time, clock)
	}
	if samplesExpected != samplesActual {
		t.Fatalf("AAC decoded samples source=%d delivered=%d", samplesExpected, samplesActual)
	}
	t.Logf("AAC PCM hash differences: count=%d indices=%v (source fully skipped priming packet excluded by HLS)", len(differing), differing)
	t.Logf("AAC decoded samples=%d duration=%.6f seconds", samplesActual, float64(samplesActual)/44100)
	t.Logf("0:a:0: %d decoded frames; shared initial mux shift %.6f seconds", len(actual), clock)
}

func copiedDecodedSampleTotal(frames []copiedDecodedFrame) int {
	total := 0
	for _, frame := range frames {
		total += frame.samples
	}
	return total
}

// Initial and refilled delivery must have identical decoder history and output,
// independently of source-container priming metadata.
func assertCopiedRefillDecode(t *testing.T, ctx context.Context, ffmpeg string, initial, refilled []byte, audio bool) {
	t.Helper()
	directory := t.TempDir()
	streams := []string{"0:v:0"}
	if audio {
		streams = append(streams, "0:a:0")
	}
	paths := []string{filepath.Join(directory, "initial.mp4"), filepath.Join(directory, "refilled.mp4")}
	for index, data := range [][]byte{initial, refilled} {
		if err := os.WriteFile(paths[index], data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, stream := range streams {
		expected := copiedDecodedFrames(t, ctx, ffmpeg, paths[0], stream)
		actual := copiedDecodedFrames(t, ctx, ffmpeg, paths[1], stream)
		if len(expected) != len(actual) {
			t.Fatalf("%s refill decoded frame count changed", stream)
		}
		maxDelta := float64(0)
		for index := range expected {
			delta := math.Abs(expected[index].time - actual[index].time)
			maxDelta = math.Max(maxDelta, delta)
			if expected[index].hash != actual[index].hash || expected[index].samples != actual[index].samples || delta > 0.001001 {
				t.Fatalf("%s refill changed decoded frame %d: initial=%+v refilled=%+v", stream, index, expected[index], actual[index])
			}
		}
		t.Logf("%s initial/refill decoded hashes and samples identical: %d frames; maximum PTS delta %.6f seconds (one MKV source tick)", stream, len(actual), maxDelta)
	}
}
