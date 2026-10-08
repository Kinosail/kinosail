package server

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// These written-first controls certify complete input parsing and integer
// association only. They never grant generated-cache or process admission.
func TestRemainingNonKeyCompleteSourceAudioClock(t *testing.T) {
	cases := []struct {
		name                 string
		denominator, leading int64
		firstPTS, targetPTS  int64
		firstNative, target  int64
		edit                 int64
	}{
		{"millisecond-full", 1000, 1024, 11925, 599040, 572416, 599040, 27584},
		{"sample-full", 48000, 1024, 572416, 599040, 572416, 599040, 27584},
		{"sample-leading16", 48000, 16, 571400, 599048, 571408, 599056, 28600},
	}
	for _, value := range cases {
		t.Run(value.name, func(t *testing.T) {
			fixture := remainingNonKeySourceAudio(t, value.denominator, value.leading)
			proof, err := deriveCopiedHLSSourceAudio(t.Context(), fixture.encode(t), []byte(fixture.normalized), fixture.first, 12_500_000, fixture.edit)
			if err != nil || proof == nil {
				t.Fatal("nonkey complete source-clock producer missing")
			}
			remainingNonKeySourceAudioCorrespondence(t, proof, fixture.first, []int64{
				value.firstPTS, value.firstNative, value.targetPTS, value.target, value.edit, value.leading,
			})
			mapping := &copiedHLSPresentation{
				RequestedMicros: 12_500_000,
				Proof:           &copiedHLSPresentationProof{Audio: proof},
			}
			if !validCopiedHLSAudioProof(mapping) {
				t.Fatal("nonkey derived source-clock tuple invalid")
			}
		})
	}
}

func remainingNonKeySourceAudioInteriorDamages() []remainingNonKeySourceAudioDamage {
	return []remainingNonKeySourceAudioDamage{
		{"interior-frame-gap", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[9]["pts"] = 195
		}},
		{"late-frame-gap", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[900]["pts"] = 19201
		}},
		{"interior-samples", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[9]["nb_samples"] = 1023
		}},
		{"interior-packet-gap", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[10]["pts"] = 195
		}},
		{"interior-packet-duration", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[10]["duration"] = 20
		}},
		{"interior-packet-dts", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[10]["dts"] = 195
		}},
		{"interior-delay", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "frame")[9]["side_data_list"] = []map[string]any{{"side_data_type": "Skip Samples", "skip_samples": 1}}
		}},
		{"discard-padding", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[0]["side_data_list"].([]map[string]any)[0]["discard_padding"] = 1
		}},
		{"unsupported-skip", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[0]["side_data_list"].([]map[string]any)[0]["skip_samples"] = 1008
		}},
		{"missing-prime", func(f *remainingNonKeySourceAudioFixture) {
			f.native["packets_and_frames"] = f.native["packets_and_frames"].([]map[string]any)[1:]
		}},
		{"missing-packet-hash", func(f *remainingNonKeySourceAudioFixture) {
			delete(remainingNonKeySourceRows(*f, "packet")[560], "data_hash")
		}},
		{"ambiguous-packet", func(f *remainingNonKeySourceAudioFixture) {
			remainingNonKeySourceRows(*f, "packet")[561]["data_hash"] = remainingNonKeySourceRows(*f, "packet")[560]["data_hash"]
		}},
		{"missing-private-association", func(f *remainingNonKeySourceAudioFixture) { f.first = [32]byte{1} }},
		{"normalized-interior-gap", func(f *remainingNonKeySourceAudioFixture) {
			f.normalized = strings.Replace(f.normalized, "0, 9216, 9216,", "0, 9217, 9217,", 1)
		}},
		{"normalized-conflicting-header", func(f *remainingNonKeySourceAudioFixture) {
			f.normalized += "#tb 0: 1/1000\n"
		}},
		{"normalized-truncated", func(f *remainingNonKeySourceAudioFixture) {
			f.normalized = f.normalized[:strings.LastIndex(strings.TrimSuffix(f.normalized, "\n"), "\n")+1]
		}},
		{"native-truncated", func(f *remainingNonKeySourceAudioFixture) {
			rows := f.native["packets_and_frames"].([]map[string]any)
			f.native["packets_and_frames"] = rows[:len(rows)-2]
		}},
		{"edit-outside-limit", func(f *remainingNonKeySourceAudioFixture) { f.edit = 28000 }},
	}
}

func TestRemainingNonKeySourceAudioDamagedInterior(t *testing.T) {
	for _, value := range remainingNonKeySourceAudioInteriorDamages() {
		t.Run(value.name, func(t *testing.T) {
			fixture := remainingNonKeySourceAudio(t, 1000, 1024)
			value.damage(&fixture)
			if _, err := deriveCopiedHLSSourceAudio(t.Context(), fixture.encode(t), []byte(fixture.normalized), fixture.first, 12_500_000, fixture.edit); err == nil {
				t.Fatal("nonkey damaged complete source-clock admitted")
			}
		})
	}
}

func TestRemainingNonKeySourceAudioAmbiguityAndCancellation(t *testing.T) {
	fixture := remainingNonKeySourceAudio(t, 1000, 1024)
	data := fixture.encode(t)
	for _, value := range []struct {
		name   string
		source []byte
	}{
		{"duplicate-json", []byte(strings.Replace(string(data), "\"channels\":2", "\"channels\":2,\"channels\":2", 1))},
		{"trailing-json", append(append([]byte{}, data...), []byte("{}")...)},
		{"native-byte-limit", []byte(strings.Repeat(" ", (2<<20)+1))},
		{"empty-normalized", data},
	} {
		t.Run(value.name, func(t *testing.T) {
			normalized := []byte(fixture.normalized)
			if value.name == "empty-normalized" {
				normalized = nil
			}
			if _, err := deriveCopiedHLSSourceAudio(t.Context(), value.source, normalized, fixture.first, 12_500_000, fixture.edit); err == nil {
				t.Fatal("nonkey ambiguous complete source-clock admitted")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	deadline, release := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer release()
	for _, expired := range []context.Context{ctx, deadline} {
		if _, err := deriveCopiedHLSSourceAudio(expired, data, []byte(fixture.normalized), fixture.first, 12_500_000, fixture.edit); err == nil {
			t.Fatal("nonkey canceled source-clock association admitted")
		}
	}
}

func remainingNonKeySourceAudioCorrespondence(t *testing.T, proof *copiedHLSAudioProof, packet [32]byte, expected []int64) {
	t.Helper()
	actual := []int64{proof.FirstPTS, proof.FirstNativeSample, proof.TargetPTS, proof.TargetNativeSample, proof.MediaTime, proof.LeadingSamples}
	if !slices.Equal(actual, expected) || proof.FirstPacket != packet || proof.SourceClock == [32]byte{} {
		t.Fatal("nonkey complete source-clock integer correspondence")
	}
}

// Reject requests outside the bounded source-prefix and integer sample grid.
func TestRemainingNonKeySourceAudioRequestedEligibility(t *testing.T) {
	fixture := remainingNonKeySourceAudio(t, 1000, 1024)
	data := fixture.encode(t)
	for _, micros := range []int64{-1, 0, 20_000_001, 12_500_001} {
		if _, err := deriveCopiedHLSSourceAudio(t.Context(), data, []byte(fixture.normalized), fixture.first, micros, fixture.edit); err == nil {
			t.Fatalf("nonkey ineligible source-clock request admitted micros=%d", micros)
		}
	}
}
