package hlsmanifest

import (
	"math"
	"strings"
	"testing"
)

func TestCompleteVODPreservesGuardedManifests(t *testing.T) {
	const event = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2,\nsegment-00000.m4s\n"
	for _, test := range []struct {
		name, manifest string
		duration       float64
	}{
		{"already VOD", strings.ReplaceAll(event, ":EVENT", ":VOD"), 4},
		{"zero duration", event, 0},
		{"negative duration", event, -1},
		{"nan duration", event, math.NaN()},
		{"infinite duration", event, math.Inf(1)},
		{"duration over seven days", event, 7*24*60*60 + 1},
		{"segment count limit", strings.ReplaceAll(event, "#EXTINF:2,", "#EXTINF:0.001,"), 101},
		{"established segment count limit", event, 200002},
		{"missing cadence", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n", 4},
		{"zero cadence", strings.ReplaceAll(event, "#EXTINF:2,", "#EXTINF:0,"), 4},
		{"oversized cadence", strings.ReplaceAll(event, "#EXTINF:2,", "#EXTINF:61,"), 100},
		{"malformed cadence", strings.ReplaceAll(event, "#EXTINF:2,", "#EXTINF:no,"), 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, _ := CompleteVOD([]byte(test.manifest), test.duration, 2)
			if got := string(result); got != test.manifest {
				t.Fatalf("guarded manifest changed: %q", got)
			}
		})
	}
}

func TestCompleteVODRejectsOversizedObservedPrefix(t *testing.T) {
	const segment = "#EXTINF:2,\nsegment.m4s\n"
	manifest := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n" + strings.Repeat(segment, 100_001)
	result, ready := CompleteVOD([]byte(manifest), 200_002, 2)
	if ready || string(result) != manifest {
		t.Fatal("oversized observed prefix projected a timeline")
	}
}

func TestCompleteVODRejectsNegativeProjection(t *testing.T) {
	const manifest = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2,\nsegment-00000.m4s\n#EXTINF:2,\nsegment-00001.m4s\n"
	result, ready := CompleteVOD([]byte(manifest), 1, 2)
	if ready || string(result) != manifest {
		t.Fatal("duration shorter than the observed prefix projected a timeline")
	}
}

func TestCompleteVODAllowsSegmentLimit(t *testing.T) {
	const header = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n"
	const segment = "#EXTINF:2,\nsegment.m4s\n"
	for name, manifest := range map[string]string{
		"observed":  header + strings.Repeat(segment, 100_000),
		"projected": header + "#EXTINF:2,\nsegment-00000.m4s\n",
	} {
		t.Run(name, func(t *testing.T) {
			result, ready := CompleteVOD([]byte(manifest), 200_000, 2)
			if !ready || strings.Count(string(result), "#EXTINF:") != 100_000 {
				t.Fatal("timeline at the segment limit was rejected")
			}
		})
	}
}

func TestCompleteVODUsesPlayerCadenceAndCompletedTimeline(t *testing.T) {
	const event = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.1,\nsegment-00000.m4s\n#EXTINF:3.9,\nsegment-00001.m4s\n"
	result, ready := CompleteVOD([]byte(event), 8.1, 4)
	got := string(result)
	const want = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:4.1,\nsegment-00000.m4s\n#EXTINF:3.9,\nsegment-00001.m4s\n#EXTINF:0.100000,\nsegment-00002.m4s\n#EXT-X-ENDLIST\n"
	if !ready || got != want {
		t.Fatalf("projected timeline = %q", got)
	}
	completed := event + "#EXT-X-DISCONTINUITY\n#EXTINF:1.2,\nsegment-00002.m4s\n#EXT-X-ENDLIST\n"
	result, ready = CompleteVOD([]byte(completed), 99, 4)
	if got := string(result); !ready || got != strings.ReplaceAll(completed, ":EVENT", ":VOD") {
		t.Fatalf("completed timeline changed: %q", got)
	}
}

func TestCompleteVODUsesRepeatedObservedCadence(t *testing.T) {
	const event = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4,\nsegment-00000.m4s\n#EXTINF:4,\nsegment-00001.m4s\n"
	result, ready := CompleteVOD([]byte(event), 304, 2)
	if got := string(result); !ready || !strings.Contains(got, "#EXTINF:4.000000,\nsegment-00075.m4s") {
		t.Fatalf("observed cadence projection = %q, ready=%t", got, ready)
	}
}

func TestHLSDurationValidationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		duration float64
		invalid  bool
	}{
		{"zero", 0, true},
		{"negative", -1, true},
		{"nan", math.NaN(), true},
		{"positive infinity", math.Inf(1), true},
		{"negative infinity", math.Inf(-1), true},
		{"smallest positive", math.SmallestNonzeroFloat64, false},
		{"maximum", 7 * 24 * 60 * 60, false},
		{"over maximum", 7*24*60*60 + 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := invalidHLSVODDuration(test.duration); got != test.invalid {
				t.Fatalf("invalidHLSVODDuration(%v) = %t, want %t", test.duration, got, test.invalid)
			}
		})
	}
}

func TestHLSSegmentDurationValidationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		duration float64
		invalid  bool
	}{
		{"zero", 0, true},
		{"negative", -1, true},
		{"nan", math.NaN(), true},
		{"positive infinity", math.Inf(1), true},
		{"negative infinity", math.Inf(-1), true},
		{"smallest positive", math.SmallestNonzeroFloat64, false},
		{"maximum", 60, false},
		{"over maximum", 61, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := invalidHLSSegmentDuration(test.duration); got != test.invalid {
				t.Fatalf("invalidHLSSegmentDuration(%v) = %t, want %t", test.duration, got, test.invalid)
			}
		})
	}
}

func TestObservedVODPrefixRequiresASegment(t *testing.T) {
	for _, test := range []struct {
		name     string
		manifest string
		valid    bool
	}{
		{"empty", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n", false},
		{"one segment", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2,\nsegment-00000.m4s\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, valid := observedVODPrefix([]byte(test.manifest), 2)
			if valid != test.valid {
				t.Fatalf("observedVODPrefix validity = %t, want %t", valid, test.valid)
			}
		})
	}
}
