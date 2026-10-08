package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A public ready response cannot expose forged interior audio-clock metadata.
// These isolated controls protect strict persisted sample/phase equations; they
// are synthetic schemas, not codec, packet, certificate or renderer evidence.
func TestRemainingNonKeyAudioProofMetadata(t *testing.T) {
	for _, container := range []string{"mkv", "mp4"} {
		t.Run(container, func(t *testing.T) {
			value := remainingNonKeyContractObject(12.5, true)
			value["Presentation"].(map[string]any)["Proof"].(map[string]any)["Audio"] = remainingNonKeyAudioProofObject(container)
			var timeline copiedHLSTimeline
			remainingNonKeyContractDecode(t, value, &timeline)
			if !validCopiedHLSTimeline(&timeline) {
				t.Fatal("nonkey coherent audio-proof metadata rejected")
			}
		})
	}
	cases := []struct {
		name, field string
		value       any
	}{
		{"codec", "Codec", "opus"},
		{"profile", "Profile", "HE-AAC"},
		{"rate", "SampleRate", 44100},
		{"channels", "Channels", 1},
		{"numerator", "Numerator", 2},
		{"denominator-zero", "Denominator", 0},
		{"denominator-unknown", "Denominator", 48001},
		{"source-clock-missing", "SourceClock", [32]byte{}},
		{"packet-missing", "FirstPacket", [32]byte{}},
		{"source-pts", "FirstPTS", -1},
		{"native-origin", "FirstNativeSample", -1},
		{"first-native-gap", "FirstNativeSample", 573440},
		{"requested-sample", "RequestedSample", 600001},
		{"target-after-request", "TargetPTS", 700000},
		{"target-native-gap", "TargetNativeSample", 599041},
		{"sample-count", "TargetSamples", 1023},
		{"leading-frame", "LeadingSamples", 32},
		{"phase", "SourcePhase", 1},
		{"edit-one-sample", "MediaTime", 27585},
		{"delta-bound", "OriginalMediaTime", 27617},
	}
	for _, damage := range cases {
		t.Run(damage.name, func(t *testing.T) {
			value := remainingNonKeyContractObject(12.5, true)
			audio := remainingNonKeyAudioProofObject("mkv")
			audio[damage.field] = damage.value
			value["Presentation"].(map[string]any)["Proof"].(map[string]any)["Audio"] = audio
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal("nonkey audio-proof fixture encoding")
			}
			var timeline copiedHLSTimeline
			if json.Unmarshal(data, &timeline) == nil && validCopiedHLSTimeline(&timeline) {
				t.Fatalf("nonkey damaged audio-proof admitted field=%s", damage.field)
			}
		})
	}
}

func remainingNonKeyAudioProofObject(container string) map[string]any {
	clock, packet := [32]byte{1}, [32]byte{2}
	value := map[string]any{"Codec": "aac", "Profile": "LC", "SampleRate": 48000, "Channels": 2,
		"Numerator": 1, "Denominator": 1000, "SourceClock": clock, "FirstPacket": packet,
		"FirstPTS": 11925, "FirstNativeSample": 572416, "RequestedSample": 600000,
		"TargetPTS": 599040, "TargetNativeSample": 599040, "TargetSamples": 1024,
		"LeadingSamples": 1024, "SourcePhase": 0, "MediaTime": 27584, "OriginalMediaTime": 27600}
	if container == "mp4" {
		for field, replacement := range map[string]any{"Denominator": 48000, "FirstPTS": 571400,
			"FirstNativeSample": 571408, "TargetPTS": 599048, "TargetNativeSample": 599056,
			"LeadingSamples": 16, "SourcePhase": -8, "MediaTime": 28600, "OriginalMediaTime": 28600} {
			value[field] = replacement
		}
	}
	return value
}

// The actual producer needs stronger source-row, skip/delay, process/deadline,
// private-packet, root/generation and asset binding. This metadata test does not
// certify those operations or allow naked geometry to become publicly ready.
func TestRemainingNonKeyAudioProofDoesNotGrantCacheReadiness(t *testing.T) {
	for _, container := range []string{"mkv", "mp4"} {
		t.Run(fmt.Sprintf("synthetic-%s", container), func(t *testing.T) {
			_, directory, _ := remainingNonKeyAssetFixture(t)
			value := remainingNonKeyContractObject(12.5, true)
			value["Presentation"].(map[string]any)["Proof"].(map[string]any)["Audio"] = remainingNonKeyAudioProofObject(container)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal("nonkey synthetic audio metadata encoding")
			}
			remainingNonKeyAssetWrite(t, directory, ".source", []byte("owned-source-policy"))
			remainingNonKeyAssetWrite(t, directory, ".copy-timeline", data)
			manager := &hlsManager{ctx: t.Context(), cache: filepath.Dir(directory)}
			if _, err := manager.readCopiedHLSTimelineContext(t.Context(), directory, "owned-source-policy"); err == nil {
				t.Fatal("nonkey synthetic audio proof granted naked cache readiness")
			}
			after, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
			if err != nil || !bytes.Equal(after, data) {
				t.Fatal("nonkey synthetic audio proof rejection mutated metadata")
			}
		})
	}
}
