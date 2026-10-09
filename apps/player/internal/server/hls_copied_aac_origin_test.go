package server

import (
	"strings"
	"testing"
)

// Gap: public media cannot inject arbitrary integer clocks or an ambiguous first
// payload. See the pre-implementation failure matrix; these are not E2E claims.
func TestCopiedAACFirstSourceWitness(t *testing.T) {
	hash := "SHA256:" + strings.Repeat("a", 64)
	first := copiedHLSAudioPacket{PTS: 571400, DTS: 571400, Duration: 1024, Hash: hash}
	source := []copiedHLSAudioPacket{first, {PTS: 572424, DTS: 572424, Duration: 1024, Hash: "SHA256:" + strings.Repeat("b", 64)}}
	raw := copiedHLSAudioPacket{Duration: 1024, Hash: hash}
	edited := raw
	edited.PTS, edited.DTS = -4600, -4600
	origin, err := copiedHLSAudioWitness(source, edited, raw, 12000000)
	if err != nil || origin.Physical != -571400 || origin.Edit != 4600 || origin.FirstPTS != first.PTS || origin.FirstHash != hash {
		t.Fatalf("source first-packet origin = %#v, %v", origin, err)
	}
	for _, damage := range []string{"absent", "ambiguous", "raw-clock", "wrong-duration", "wrong-edit", "wrong-seek", "wrong-hash", "source-dts", "raw-dts", "edited-dts", "distinct-clock-ambiguity"} {
		t.Run(damage, func(t *testing.T) {
			rows := append([]copiedHLSAudioPacket(nil), source...)
			n, r, seek := edited, raw, int64(12000000)
			changes := map[string]func(){
				"absent":         func() { rows = rows[1:] },
				"ambiguous":      func() { rows = append(rows, first) },
				"raw-clock":      func() { r.PTS, r.DTS = 1, 1 },
				"wrong-duration": func() { n.Duration = 1000 },
				"wrong-edit":     func() { n.PTS, n.DTS = -4601, -4601 },
				"wrong-seek":     func() { seek += 21 },
				"wrong-hash":     func() { n.Hash = "SHA256:" + strings.Repeat("c", 64) },
				"source-dts":     func() { rows[0].DTS-- },
				"raw-dts":        func() { r.DTS++ },
				"edited-dts":     func() { n.DTS++ },
				"distinct-clock-ambiguity": func() {
					other := first
					other.PTS, other.DTS = first.PTS+2048, first.DTS+2048
					rows = append(rows, other)
				},
			}
			changes[damage]()
			if _, err := copiedHLSAudioWitness(rows, n, r, seek); err == nil {
				t.Fatal("uncertified first packet acquired an origin")
			}
		})
	}
}

func TestCopiedAACActualRefillRescaleClosesRoundingCarry(t *testing.T) {
	for _, sample := range []struct {
		name                          string
		physical, seek, mux, expected int64
	}{
		{"normal-initial", -571400, 20000000, 8000000, 4600},
		{"original-clock", -571400, 20000000, 8095875, -2},
		{"fractional-carry", -571401, 20000022, 8000011, 4599},
		{"fractional-opposite", -571401, 20000011, 8000000, 4600},
	} {
		t.Run(sample.name, func(t *testing.T) {
			shift, err := copiedHLSAudioShift(sample.physical, sample.seek, sample.mux)
			if err != nil || shift != sample.expected {
				t.Fatalf("shift = %d, %v, want %d", shift, err, sample.expected)
			}
		})
	}
	for _, sample := range [][3]int64{{1 << 52, 1, 1}, {-1, -1, 0}, {-1, 0, -1}, {-1, 604800000001, 0}} {
		if _, err := copiedHLSAudioShift(sample[0], sample[1], sample[2]); err == nil {
			t.Fatal("unbounded origin or actual mux values admitted")
		}
	}
}

func TestCopiedAACCertificateVersionFollowsOrigin(t *testing.T) {
	if copiedHLSCertificateVersion(&copiedHLSTimeline{}) != 1 {
		t.Fatal("legacy producer changed")
	}
	if copiedHLSCertificateVersion(&copiedHLSTimeline{AudioOrigin: &copiedHLSAudioOrigin{}}) != 2 {
		t.Fatal("new origin reused Version1")
	}
}

func TestCopiedAACPendingInitialAndBoundRefillRemainDistinct(t *testing.T) {
	timeline := &copiedHLSTimeline{Policy: "test:copied-aac=2", Strategy: "h264-idr-keys-1", Numerator: 1, Denominator: 16000, TimeBase: 1.0 / 16000, Keys: []copiedHLSKey{{PTS: 0, DTS: -1333}, {PTS: 32000, DTS: 30667}}, End: 4, AudioOrigin: &copiedHLSAudioOrigin{SourceTrack: 1}}
	if !validCopiedAACOrigin(timeline) {
		t.Fatal("source-qualified initial generation could not bootstrap")
	}
	if _, err := copiedHLSProducerArguments([]string{"-c:a", "copy"}, timeline, 1); err == nil {
		t.Fatal("pending initial source eligibility acquired refill admission")
	}
	clock := 0.0
	timeline.Clock = &clock
	if validCopiedAACOrigin(timeline) {
		t.Fatal("empty origin acquired a bound certificate")
	}
	timeline.AudioOrigin.FirstHash = "SHA256:" + strings.Repeat("a", 64)
	if !validCopiedAACOrigin(timeline) {
		t.Fatal("complete zero-origin contract rejected")
	}
	timeline.AudioOrigin.InitialSeekMicros = 12000000
	if validCopiedAACOrigin(timeline) {
		t.Fatal("different initial input seek inherited first-key geometry")
	}
}

func TestCopiedAACPolicyCannotCarryLegacyOriginKind(t *testing.T) {
	clock := 0.0
	timeline := &copiedHLSTimeline{Policy: "test:copied-aac=2", Strategy: "h264-idr-keys-1", Numerator: 1, Denominator: 1000, TimeBase: 0.001, Keys: []copiedHLSKey{{PTS: 0, DTS: 0}}, End: 2, Clock: &clock}
	if validCopiedHLSTimeline(timeline) {
		t.Fatal("P2 producer policy admitted a missing origin and Version1 dispatch")
	}
	timeline.Policy = "legacy"
	if !validCopiedHLSTimeline(timeline) {
		t.Fatal("unqualified legacy contract changed")
	}
}
