package server_test

import (
	"context"
	"encoding/json"
	"math"
	"os/exec"
	"strconv"
	"testing"
)

func assertCopiedAudioPackets(t *testing.T, ctx context.Context, source, delivered string, clock float64) copiedAudioProbe {
	t.Helper()
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	sourceProbe := copiedAudioPackets(t, ctx, ffprobe, source)
	actual := copiedAudioPackets(t, ctx, ffprobe, delivered).Packets
	expected := audibleCopiedSourcePackets(t, sourceProbe)
	assertCopiedAudioPacketCount(t, expected, actual)
	for index := range expected {
		if actual[index].Hash != expected[index].Hash {
			t.Fatalf("AAC payload discontinuity at audible packet %d", index)
		}
		assertCopiedAudioPacketClock(t, index, expected[index], actual[index], clock)
	}
	t.Logf("AAC: %d identical ordered audible compressed packets; fully skipped priming packet excluded", len(actual))
	return sourceProbe
}

type copiedAudioPacket struct {
	PTS      string `json:"pts_time"`
	DTS      string `json:"dts_time"`
	Duration string `json:"duration_time"`
	Hash     string `json:"data_hash"`
	Side     []struct {
		Skip    int `json:"skip_samples"`
		Discard int `json:"discard_padding"`
	} `json:"side_data_list"`
}

type copiedAudioProbe struct {
	Packets []copiedAudioPacket `json:"packets"`
	Streams []struct {
		SampleRate string `json:"sample_rate"`
		Padding    int    `json:"initial_padding"`
	} `json:"streams"`
}

func copiedAudioPackets(t *testing.T, ctx context.Context, ffprobe, path string) copiedAudioProbe {
	t.Helper()
	command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "a:0", "-show_packets", "-show_data_hash", "sha256", "-show_entries", "packet=pts_time,dts_time,duration_time,data_hash,side_data_list:packet_side_data=skip_samples,discard_padding:stream=sample_rate,initial_padding", "-of", "json", path) //nolint:gosec // Discovered tool and generated fixture.
	data, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var value copiedAudioProbe
	if json.Unmarshal(data, &value) != nil {
		t.Fatal("invalid audio packet metadata")
	}
	return value
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

func audibleCopiedSourcePackets(t *testing.T, probe copiedAudioProbe) []copiedAudioPacket {
	t.Helper()
	if len(probe.Streams) != 1 || probe.Streams[0].SampleRate != "44100" || probe.Streams[0].Padding != 1024 || len(probe.Packets) < 2 {
		t.Fatal("source fixture must retain explicit 1024-sample AAC encoder padding")
	}
	first := probe.Packets[0]
	if len(first.Side) > 1 {
		t.Fatal("ambiguous AAC priming side data")
	}
	if len(first.Side) == 1 && (first.Side[0].Skip != 1024 || first.Side[0].Discard != 0) {
		t.Fatal("AAC packet skip metadata conflicts with explicit encoder padding")
	}
	assertCopiedPrimerClock(t, first, probe.Packets[1])
	return probe.Packets[1:]
}

func assertCopiedPrimerClock(t *testing.T, first, next copiedAudioPacket) {
	t.Helper()
	// FFprobe6 reports Matroska CodecDelay as stream initial_padding. FFprobe9
	// also exposes Skip Samples on this packet. Both explicitly discard the same
	// entire negative-PTS packet; timestamps alone never permit an exclusion.
	pts := copiedFiniteAudioTime(t, first.PTS)
	dts := copiedFiniteAudioTime(t, first.DTS)
	duration := copiedFiniteAudioTime(t, first.Duration)
	if pts >= 0 || dts != pts || math.Abs(duration-1024.0/44100) > 0.001001 || math.Abs(pts+duration) > 0.001001 {
		t.Fatal("AAC initial padding does not fully cover the first negative packet")
	}
	if next := copiedFiniteAudioTime(t, next.PTS); next < 0 || next > 0.001001 {
		t.Fatal("AAC audible packet sequence does not start at zero")
	}
}

func copiedFiniteAudioTime(t *testing.T, value string) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		t.Fatal("malformed AAC packet timestamp or duration")
	}
	return parsed
}
