package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func transcodeRefillAudio(t *testing.T, ctx context.Context, ffmpeg, ffprobe, label string, parts [][]byte) (int, []refillAudioPacket) {
	t.Helper()
	arguments := []string{"-nostdin", "-v", "error", "-i", "pipe:0", "-map", "0:a:0", "-c:a", "pcm_s16le", "-f", "s16le", "-"}
	command := exec.CommandContext(ctx, ffmpeg, arguments...) //nolint:gosec // Fixed decoder arguments and owned synthetic public HTTP fragments.
	command.Stdin = bytes.NewReader(bytes.Join(parts, nil))
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	pcm, err := command.Output()
	status := -1
	if command.ProcessState != nil {
		status = command.ProcessState.ExitCode()
	}
	transcodeRefillLog(t, map[string]any{
		"audioDecode": label, "arguments": arguments, "exit": status,
		"pcmBytes": len(pcm), "pcmSHA256": fmt.Sprintf("%x", sha256.Sum256(pcm)),
		"diagnosticBytes": diagnostics.Len(), "diagnosticSHA256": fmt.Sprintf("%x", sha256.Sum256(diagnostics.Bytes())),
	})
	// This proves actual audio delivery, not sample alignment or waveform equality.
	if err != nil || len(pcm) == 0 || diagnostics.Len() != 0 {
		t.Fatal("public audio fragment sequence did not decode cleanly")
	}
	return len(pcm), transcodeRefillAudioPackets(t, ctx, ffprobe, label, parts)
}

type refillAudioPacket struct {
	PTS, DTS, Duration *int64
	Flags              string
	SideData           []struct {
		SkipSamples    int64 `json:"skip_samples"`
		DiscardPadding int64 `json:"discard_padding"`
	} `json:"side_data_list"`
}

func transcodeRefillAudioPackets(t *testing.T, ctx context.Context, ffprobe, label string, parts [][]byte) []refillAudioPacket {
	t.Helper()
	arguments := []string{"-v", "error", "-select_streams", "a:0", "-show_entries", "stream=time_base,sample_rate,channels:packet=pts,dts,duration,flags:packet_side_data=skip_samples,discard_padding", "-of", "json", "pipe:0"}
	command := exec.CommandContext(ctx, ffprobe, arguments...) //nolint:gosec // Fixed metadata projection of owned public HTTP fragments.
	command.Stdin = bytes.NewReader(bytes.Join(parts, nil))
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	status := -1
	if command.ProcessState != nil {
		status = command.ProcessState.ExitCode()
	}
	transcodeRefillLog(t, map[string]any{
		"audioProbe": label, "arguments": arguments, "exit": status,
		"diagnosticBytes": diagnostics.Len(), "diagnosticSHA256": fmt.Sprintf("%x", sha256.Sum256(diagnostics.Bytes())),
	})
	if err != nil || diagnostics.Len() != 0 {
		t.Fatal("audio packet clock probe failed")
	}
	packets := transcodeRefillAudioPacketClock(t, output)
	var total int64
	var gaps []map[string]int64
	trimmed := make(map[int]refillAudioPacket)
	for index, packet := range packets {
		total += *packet.Duration
		if index > 0 {
			previous := packets[index-1]
			if gap := *packet.DTS - *previous.DTS - *previous.Duration; gap != 0 {
				gaps = append(gaps, map[string]int64{"beforePacket": int64(index), "ticks": gap})
			}
		}
		if strings.Contains(packet.Flags, "D") || len(packet.SideData) > 0 {
			trimmed[index] = packet
		}
	}
	transcodeRefillLog(t, map[string]any{
		"audioPackets": label, "count": len(packets), "totalDurationTicks": total,
		"timeBase": "1/48000", "channels": 2, "first": packets[0], "last": packets[len(packets)-1],
		"discontinuities": gaps, "trimming": trimmed, "projectionSHA256": fmt.Sprintf("%x", sha256.Sum256(output)),
	})
	return packets
}

func transcodeRefillAudioPacketClock(t *testing.T, output []byte) []refillAudioPacket {
	t.Helper()
	var decoded struct {
		Packets []refillAudioPacket
		Streams []struct {
			TimeBase   string `json:"time_base"`
			SampleRate string `json:"sample_rate"`
			Channels   int
		}
	}
	if json.Unmarshal(output, &decoded) != nil || len(decoded.Streams) != 1 || len(decoded.Packets) == 0 || len(decoded.Packets) > 512 {
		t.Fatal("audio packet projection is missing or outside the fixture bounds")
	}
	stream := decoded.Streams[0]
	if stream.TimeBase != "1/48000" || stream.SampleRate != "48000" || stream.Channels != 2 {
		t.Fatal("audio fixture must retain its 48 kHz stereo clock")
	}
	for _, packet := range decoded.Packets {
		transcodeRefillAudioPacketFields(t, packet)
	}
	return decoded.Packets
}

func transcodeRefillAudioPacketFields(t *testing.T, packet refillAudioPacket) {
	t.Helper()
	if packet.PTS == nil || packet.DTS == nil || packet.Duration == nil {
		t.Fatal("audio packet clock fields are missing")
	}
	if *packet.Duration <= 0 || !regexp.MustCompile(`^[A-Z_]{1,8}$`).MatchString(packet.Flags) || len(packet.SideData) > 4 {
		t.Fatal("audio packet duration or metadata is outside fixture bounds")
	}
}

func transcodeRefillCompareAudio(t *testing.T, label string, original, actual []refillAudioPacket) {
	t.Helper()
	if len(original) != len(actual) {
		t.Errorf("%s changed the AAC packet count: original=%d actual=%d", label, len(original), len(actual))
		return
	}
	for index, expected := range original {
		packet := actual[index]
		if *expected.PTS != *packet.PTS || *expected.DTS != *packet.DTS || *expected.Duration != *packet.Duration {
			t.Errorf("%s shifted AAC packet %d", label, index)
			return
		}
	}
}
