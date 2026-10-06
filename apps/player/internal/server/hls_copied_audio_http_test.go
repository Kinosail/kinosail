package server_test

import (
	"context"
	"encoding/json"
	"math"
	"os/exec"
	"strconv"
	"testing"
)

func assertCopiedAudioPackets(t *testing.T, ctx context.Context, source, delivered string, clock float64) {
	t.Helper()
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	expected := copiedAudioPackets(t, ctx, ffprobe, source)
	actual := copiedAudioPackets(t, ctx, ffprobe, delivered)
	expected = audibleCopiedSourcePackets(t, expected)
	assertCopiedAudioPacketCount(t, expected, actual)
	for index := range expected {
		if actual[index].Hash != expected[index].Hash {
			t.Fatalf("AAC payload discontinuity at audible packet %d", index)
		}
		assertCopiedAudioPacketClock(t, index, expected[index], actual[index], clock)
	}
	t.Logf("AAC: %d identical ordered audible compressed packets; fully skipped priming packet excluded", len(actual))
}

type copiedAudioPacket struct {
	PTS  string `json:"pts_time"`
	DTS  string `json:"dts_time"`
	Hash string `json:"data_hash"`
	Side []struct {
		Skip int `json:"skip_samples"`
	} `json:"side_data_list"`
}

func copiedAudioPackets(t *testing.T, ctx context.Context, ffprobe, path string) []copiedAudioPacket {
	t.Helper()
	command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "a:0", "-show_packets", "-show_data_hash", "sha256", "-show_entries", "packet=pts_time,dts_time,data_hash,side_data_list", "-of", "json", path) //nolint:gosec // Discovered tool and generated fixture.
	data, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Packets []copiedAudioPacket `json:"packets"`
	}
	if json.Unmarshal(data, &value) != nil {
		t.Fatal("invalid audio packet metadata")
	}
	return value.Packets
}

func assertCopiedAudioPacketCount(t *testing.T, expected, actual []copiedAudioPacket) {
	t.Helper()
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
}

func assertCopiedAudioPacketClock(t *testing.T, index int, source, delivered copiedAudioPacket, clock float64) {
	t.Helper()
	for _, values := range [][2]string{{source.PTS, delivered.PTS}, {source.DTS, delivered.DTS}} {
		first, err1 := strconv.ParseFloat(values[0], 64)
		second, err2 := strconv.ParseFloat(values[1], 64)
		if err1 != nil || err2 != nil || math.IsNaN(first) || math.IsNaN(second) || math.IsInf(first, 0) || math.IsInf(second, 0) || math.Abs(second-first-clock) > 0.001001 {
			t.Fatalf("AAC packet %d clock drift: source=%s delivered=%s mux=%.6f", index, values[0], values[1], clock)
		}
	}
}

func audibleCopiedSourcePackets(t *testing.T, expected []copiedAudioPacket) []copiedAudioPacket {
	t.Helper()
	sourceCount := len(expected)
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
	if sourceCount == len(expected) {
		t.Fatal("source fixture must retain its explicit fully skipped1024-sample priming packet")
	}
	return expected
}
