package server

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

// These schema/geometry controls cover distinctions that a ready response alone
// cannot establish. Physical coordinates and the old clock contract stay exact.
func TestRemainingNonKeyTypedOriginsAndCuts(t *testing.T) {
	for _, origin := range []float64{12.5, 13.5, 18.2} {
		t.Run(fmt.Sprintf("%.1f", origin), func(t *testing.T) {
			remainingNonKeyTypedOriginAndCuts(t, origin)
		})
	}
}

func remainingNonKeyTypedOriginAndCuts(t *testing.T, origin float64) {
	t.Helper()
	timeline := remainingNonKeyContractTimeline(t, origin, true)
	if !validCopiedHLSTimeline(timeline) {
		t.Fatal("nonkey typed presentation certificate rejected")
	}
	decode := math.Floor(origin/2) * 2
	if timeline.point(0) != decode || timeline.Clock != nil {
		t.Fatal("nonkey mapping reinterpreted physical keys or old Clock")
	}
	physical := remainingNonKeyPhysicalManifest(timeline, len(timeline.Keys))
	projected, valid := copiedHLSManifest(physical, timeline)
	if !valid {
		t.Fatal("nonkey certified presentation projection unavailable")
	}
	first, total := remainingNonKeyContractExtents(t, projected)
	if math.Abs(first-(decode+2-origin)) > 0.000001 || math.Abs(total-(32-origin)) > 0.000001 {
		t.Fatalf("nonkey cuts first=%f total=%f", first, total)
	}
	projection := remainingNonKeyContractProjection(timeline)
	// The physical prefix has four GOPs; presentation requires a fifth.
	prefix := remainingNonKeyPhysicalManifest(timeline, 4)
	segments, ready := startupWindowSegments(prefix, 32-origin, projection)
	if !ready || len(segments) != 5 || segments[4] != "segment-00004.m4s" {
		t.Fatalf("nonkey startup required %d cuts, ready=%t; expected five", len(segments), ready)
	}
}

func TestRemainingNonKeyPendingCannotProjectAndLegacyClockStaysStrict(t *testing.T) {
	pending := remainingNonKeyContractTimeline(t, 12.5, false)
	if !validCopiedHLSTimeline(pending) {
		t.Fatal("nonkey pending typed origin rejected before generated proof")
	}
	physical := remainingNonKeyPhysicalManifest(pending, 4)
	if _, valid := copiedHLSManifest(physical, pending); valid {
		t.Fatal("nonkey pending origin certified an inert EVENT prefix")
	}
	legacy := &copiedHLSTimeline{
		Policy: "owned-source-policy", Strategy: "h264-idr-keys-1",
		Numerator: 1, Denominator: 1000, TimeBase: 0.001,
		Keys: []copiedHLSKey{{PTS: 12000, DTS: 11917}, {PTS: 14000, DTS: 13917}}, End: 16,
	}
	clock := -0.5
	legacy.Clock = &clock
	if validCopiedHLSTimeline(legacy) {
		t.Fatal("nonkey work admitted a negative legacy Clock")
	}
	clock = 0
	if !validCopiedHLSTimeline(legacy) {
		t.Fatal("nonkey work rejected the unchanged exact-key strategy")
	}
}

func TestRemainingNonKeyOriginDamageRejects(t *testing.T) {
	for _, damage := range []string{"missing", "exact", "before", "next", "eof", "decode", "old-clock", "signed-clock", "legacy-with-origin", "unknown-strategy"} {
		t.Run(damage, func(t *testing.T) {
			value := remainingNonKeyContractObject(12.5, true)
			mapping := value["Presentation"].(map[string]any)
			proof := mapping["Proof"].(map[string]any)
			remainingNonKeyDamageOrigin(value, mapping, proof, damage)
			var timeline copiedHLSTimeline
			remainingNonKeyContractDecode(t, value, &timeline)
			if validCopiedHLSTimeline(&timeline) {
				t.Fatal("nonkey damaged origin/proof remained admissible")
			}
		})
	}
}

func remainingNonKeyContractTimeline(t *testing.T, origin float64, bound bool) *copiedHLSTimeline {
	t.Helper()
	var timeline copiedHLSTimeline
	remainingNonKeyContractDecode(t, remainingNonKeyContractObject(origin, bound), &timeline)
	return &timeline
}

func remainingNonKeyContractDecode(t *testing.T, value map[string]any, timeline *copiedHLSTimeline) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil || json.Unmarshal(data, timeline) != nil {
		t.Fatal("nonkey contract fixture JSON")
	}
}

func remainingNonKeyContractObject(origin float64, bound bool) map[string]any {
	decode := int64(math.Floor(origin/2) * 2000)
	var keys []copiedHLSKey
	for point := decode; point < 32000; point += 2000 {
		keys = append(keys, copiedHLSKey{PTS: point, DTS: point - 83})
	}
	mapping := map[string]any{"RequestedMicros": int64(math.Round(origin * 1000000)), "Decode": keys[0]}
	if bound {
		mapping["Proof"] = map[string]any{
			"VideoPTS":         float64(decode)/1000 - origin,
			"VideoScale":       1000,
			"VideoMediaTime":   int64(math.Round((origin-float64(decode)/1000)*1000)) + 83,
			"VideoDecodeTime":  0,
			"VideoComposition": 83,
		}
	}
	return map[string]any{
		"Policy": "owned-source-policy", "Strategy": "h264-idr-preroll-1",
		"Numerator": 1, "Denominator": 1000, "TimeBase": 0.001, "Keys": keys, "End": 32,
		"Clock": nil, "Presentation": mapping,
	}
}

func remainingNonKeyPhysicalManifest(timeline *copiedHLSTimeline, count int) []byte {
	var value strings.Builder
	value.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for number := range count {
		fmt.Fprintf(&value, "#EXTINF:2.000000,\nsegment-%05d.m4s\n", number)
	}
	if count == len(timeline.Keys) {
		value.WriteString("#EXT-X-ENDLIST\n")
	}
	return []byte(value.String())
}

func remainingNonKeyContractExtents(t *testing.T, manifest []byte) (float64, float64) {
	t.Helper()
	first, total := -1.0, 0.0
	for _, line := range strings.Split(string(manifest), "\n") {
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		raw, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
		length, err := strconv.ParseFloat(raw, 64)
		if err != nil || length <= 0 {
			t.Fatal("nonkey projected duration")
		}
		if first < 0 {
			first = length
		}
		total += length
	}
	return first, total
}

func remainingNonKeyDamageOrigin(value, mapping, proof map[string]any, damage string) {
	origins := map[string]int64{"exact": 12000000, "before": 11999999, "next": 14000000, "eof": 32000000}
	if origin, found := origins[damage]; found {
		mapping["RequestedMicros"] = origin
		return
	}
	switch damage {
	case "missing":
		delete(value, "Presentation")
	case "decode":
		mapping["Decode"] = copiedHLSKey{PTS: 10000, DTS: 9917}
	case "old-clock":
		value["Clock"] = 0
	case "signed-clock":
		proof["VideoPTS"] = 0.5
	case "legacy-with-origin":
		value["Strategy"] = "h264-idr-keys-1"
	case "unknown-strategy":
		value["Strategy"] = "h264-idr-preroll-unknown"
	}
}

func remainingNonKeyContractProjection(timeline *copiedHLSTimeline) func([]byte) []byte {
	return func(input []byte) []byte {
		value, accepted := copiedHLSManifest(input, timeline)
		if !accepted {
			return nil
		}
		return value
	}
}
