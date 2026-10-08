package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type remainingNonKeySourceAudioFixture struct {
	native     map[string]any
	normalized string
	first      [32]byte
	edit       int64
}

func remainingNonKeySourceAudio(t *testing.T, denominator, leading int64) remainingNonKeySourceAudioFixture {
	t.Helper()
	rows := make([]map[string]any, 0, 2050)
	var output strings.Builder
	output.WriteString("#tb 0: 1/48000\n#media_type 0: audio\n#sample_rate 0: 48000\n")
	phase, skip, prime := int64(0), int64(1024), int64(-1024)
	if leading == 16 {
		phase, skip, prime = -8, 1008, -1008
	}
	packet := func(pts, duration, number int64) map[string]any {
		hash := sha256.Sum256([]byte(fmt.Sprintf("aac-payload-%d", number)))
		return map[string]any{"type": "packet", "pts": pts, "dts": pts,
			"duration": duration, "data_hash": "SHA256:" + hex.EncodeToString(hash[:])}
	}
	round := func(sample int64) int64 { return (sample*denominator + 24000) / 48000 }
	priming := packet(-round(-prime), round(1024), -1)
	priming["side_data_list"] = []map[string]any{{"side_data_type": "Skip Samples",
		"skip_samples": skip, "discard_padding": 0, "skip_reason": 0, "discard_reason": 0}}
	rows = append(rows, priming)
	var ordinal int64
	var first [32]byte
	for number := int64(0); number < 1024; number++ {
		count, clock := int64(1024), ordinal
		if number == 0 {
			count = leading
		}
		if number >= 2 {
			clock += phase
		}
		pts := round(clock)
		if leading == 1024 || number > 0 {
			duration := round(1024)
			if leading == 16 && number == 1 {
				duration = 1016
			}
			row := packet(pts, duration, number)
			rows = append(rows, row)
			if number == 559 {
				first = sha256.Sum256([]byte(fmt.Sprintf("aac-payload-%d", number)))
			}
		}
		rows = append(rows, map[string]any{"type": "frame", "pts": pts, "nb_samples": count})
		fmt.Fprintf(&output, "0, %d, %d, %d, %d, 00000000000000000000000000000000\n", clock, clock, count, count*4)
		ordinal += count
	}
	edit := int64(27600)
	if leading == 16 {
		edit = 28600
	}
	return remainingNonKeySourceAudioFixture{map[string]any{
		"streams": []map[string]any{{"codec_name": "aac", "profile": "LC",
			"sample_rate": "48000", "channels": 2, "time_base": fmt.Sprintf("1/%d", denominator)}},
		"packets_and_frames": rows,
	}, output.String(), first, edit}
}

func (fixture remainingNonKeySourceAudioFixture) encode(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(fixture.native)
	if err != nil {
		t.Fatal("nonkey source-clock fixture encoding")
	}
	return data
}

func remainingNonKeySourceRows(fixture remainingNonKeySourceAudioFixture, kind string) []map[string]any {
	var rows []map[string]any
	for _, row := range fixture.native["packets_and_frames"].([]map[string]any) {
		if row["type"] == kind {
			rows = append(rows, row)
		}
	}
	return rows
}
