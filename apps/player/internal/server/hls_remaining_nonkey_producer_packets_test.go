//go:build linux

package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
)

func remainingNonKeyProducerClock(t *testing.T, text string) float64 {
	t.Helper()
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		t.Fatal("pending producer nonfinite packet clock")
	}
	return value
}

func remainingNonKeyProducerPackets(t *testing.T, ctx context.Context, ffprobe, source, stream string) []remainingNonKeyProducerPacket {
	t.Helper()
	output := remainingNonKeyCollectorCommand(t, ctx, ffprobe, "-v", "error", "-select_streams", stream,
		"-show_packets", "-show_data_hash", "sha256", "-show_entries", "packet=pts_time,dts_time,duration_time,data_hash", "-of", "json", source)
	var result struct {
		Packets []remainingNonKeyProducerPacket `json:"packets"`
	}
	if json.Unmarshal(output, &result) != nil {
		t.Fatal("pending producer complete packet JSON")
	}
	remainingNonKeyProducerValidate(t, result.Packets)
	return result.Packets
}

func remainingNonKeyProducerValidate(t *testing.T, packets []remainingNonKeyProducerPacket) {
	t.Helper()
	if len(packets) == 0 || len(packets) > 4096 {
		t.Fatal("pending producer complete independent packet bound")
	}
	for _, packet := range packets {
		if !strings.HasPrefix(packet.Hash, "SHA256:") {
			t.Fatal("pending producer packet hash prefix")
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(packet.Hash, "SHA256:"))
		if err != nil || len(raw) != 32 || strings.ToLower(packet.Hash) != "sha256:"+strings.TrimPrefix(packet.Hash, "SHA256:") {
			t.Fatal("pending producer independent strict packet identity")
		}
		remainingNonKeyProducerClock(t, packet.PTS)
		remainingNonKeyProducerClock(t, packet.DTS)
		if remainingNonKeyProducerClock(t, packet.Duration) <= 0 {
			t.Fatal("pending producer packet duration")
		}
	}
}

func remainingNonKeyProducerEqual(t *testing.T, actual, expected []remainingNonKeyProducerPacket, stream string) {
	t.Helper()
	remainingNonKeyProducerValidate(t, expected)
	if len(actual) != len(expected) {
		t.Errorf("pending producer complete %s packet count observed=%d expected=%d", stream, len(actual), len(expected))
		return
	}
	for number, packet := range actual {
		want := expected[number]
		if packet.Hash != want.Hash {
			t.Errorf("pending producer complete %s payload sequence differs at packet=%d", stream, number)
			return
		}
		for _, clocks := range [][2]string{{packet.PTS, want.PTS}, {packet.DTS, want.DTS}, {packet.Duration, want.Duration}} {
			if math.Abs(remainingNonKeyProducerClock(t, clocks[0])-remainingNonKeyProducerClock(t, clocks[1])) > 0.000001 {
				t.Errorf("pending producer complete %s signed packet clocks differ at packet=%d", stream, number)
				return
			}
		}
	}
}
