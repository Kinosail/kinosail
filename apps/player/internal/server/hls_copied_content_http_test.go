package server_test

import (
	"context"
	"encoding/json"
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
		frames = append(frames, copiedDecodedFrame{time: float64(pts) / 1000000, hash: hash, samples: samples})
	}
	if len(frames) == 0 {
		t.Fatal("empty decoded sequence")
	}
	return frames
}

// Full-source decoding is an independent oracle for public segment continuity.
// Only a single initial mux clock is normalized, preserving all subsequent PTS.
func assertCopiedHLSContent(t *testing.T, ctx context.Context, ffmpeg, source string, media []byte, audio bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "delivered.mp4")
	if err := os.WriteFile(path, media, 0o600); err != nil {
		t.Fatal(err)
	}
	streams := []string{"0:v:0"}
	if audio {
		streams = append(streams, "0:a:0")
	}
	var videoClock float64
	for _, stream := range streams {
		if stream == "0:a:0" {
			assertCopiedAudioPackets(t, ctx, source, path, videoClock)
		}

		expected := copiedDecodedFrames(t, ctx, ffmpeg, source, stream)
		actual := copiedDecodedFrames(t, ctx, ffmpeg, path, stream)
		if len(actual) != len(expected) {
			t.Fatalf("%s decoded count = %d, source = %d", stream, len(actual), len(expected))
		}
		if stream == "0:v:0" {
			if len(actual) != 144 {
				t.Fatalf("12-second/12fps fixture decoded %d video frames", len(actual))
			}
			videoClock = actual[0].time - expected[0].time
		}
		samplesExpected, samplesActual := 0, 0
		var differingAudio []int
		for index := range expected {
			samplesExpected += expected[index].samples
			samplesActual += actual[index].samples
			if stream == "0:a:0" && actual[index].hash != expected[index].hash {
				differingAudio = append(differingAudio, index)
			}
			if stream == "0:v:0" && actual[index].hash != expected[index].hash {
				t.Fatalf("%s frame %d differs from full source", stream, index)
			}
			// MKV source packet timestamps use a 1ms time base. MP4 preserves finer
			// video/audio ticks; at most one source tick is allowed after the mux shift.
			if math.Abs(actual[index].time-expected[index].time-videoClock) > 0.001001 {
				t.Fatalf("%s frame %d timestamp drift: source %.6f, delivered %.6f, mux %.6f", stream, index, expected[index].time, actual[index].time, videoClock)
			}
		}
		if stream == "0:a:0" {
			if samplesExpected != samplesActual {
				t.Fatalf("AAC decoded samples source=%d delivered=%d", samplesExpected, samplesActual)
			}
			t.Logf("AAC PCM hash differences: count=%d indices=%v (source fully skipped priming packet excluded by HLS)", len(differingAudio), differingAudio)
			t.Logf("AAC decoded samples=%d duration=%.6f seconds", samplesActual, float64(samplesActual)/44100)
		}
		t.Logf("%s: %d decoded frames; shared initial mux shift %.6f seconds", stream, len(actual), videoClock)
	}
}

func assertCopiedAudioPackets(t *testing.T, ctx context.Context, source, delivered string, clock float64) {
	t.Helper()
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	type packet struct {
		PTS  string `json:"pts_time"`
		DTS  string `json:"dts_time"`
		Hash string `json:"data_hash"`
		Side []struct {
			Skip int `json:"skip_samples"`
		} `json:"side_data_list"`
	}
	var results [][]packet
	for _, path := range []string{source, delivered} {
		command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "a:0", "-show_packets", "-show_data_hash", "sha256", "-show_entries", "packet=pts_time,dts_time,data_hash,side_data_list", "-of", "json", path) //nolint:gosec // Discovered tool and generated fixture.
		data, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		var value struct {
			Packets []packet `json:"packets"`
		}
		if json.Unmarshal(data, &value) != nil {
			t.Fatal("invalid audio packet metadata")
		}
		results = append(results, value.Packets)
	}
	expected, actual := results[0], results[1]
	// AAC encoder delay is explicitly fully discarded by source metadata. HLS
	// omits only this negative-PTS priming packet; all audible payloads are required.
	if len(expected) > 0 && len(expected[0].Side) == 1 && expected[0].Side[0].Skip == 1024 {
		pts, err := strconv.ParseFloat(expected[0].PTS, 64)
		if err != nil || math.IsNaN(pts) || math.IsInf(pts, 0) || pts >= 0 {
			t.Fatal("AAC priming PTS is not the explicit negative source packet")
		}
		if pts < 0 {
			expected = expected[1:]
		}
	}
	if len(results[0]) == len(expected) {
		t.Fatal("source fixture must retain its explicit fully skipped1024-sample priming packet")
	}
	if len(actual) != len(expected) {
		positions := map[string]int{}
		for i, p := range expected {
			positions[p.Hash] = i
		}
		previous := -1
		for i, p := range actual {
			position, ok := positions[p.Hash]
			if !ok || position != previous+1 {
				t.Logf("AAC continuity gap at delivered %d: source %d -> %d, pts=%s", i, previous, position, p.PTS)
			}
			previous = position
		}
		t.Fatalf("audible AAC packets source=%d delivered=%d", len(expected), len(actual))
	}
	for index := range expected {
		if actual[index].Hash != expected[index].Hash {
			t.Fatalf("AAC payload discontinuity at audible packet %d", index)
		}
		for _, values := range [][2]string{{expected[index].PTS, actual[index].PTS}, {expected[index].DTS, actual[index].DTS}} {
			first, err1 := strconv.ParseFloat(values[0], 64)
			second, err2 := strconv.ParseFloat(values[1], 64)
			if err1 != nil || err2 != nil || math.IsNaN(first) || math.IsNaN(second) || math.IsInf(first, 0) || math.IsInf(second, 0) || math.Abs(second-first-clock) > 0.001001 {
				t.Fatalf("AAC packet %d clock drift: source=%s delivered=%s mux=%.6f", index, values[0], values[1], clock)
			}
		}
	}
	t.Logf("AAC: %d identical ordered audible compressed packets; fully skipped priming packet excluded", len(actual))
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
