package server

import (
	"reflect"
	"testing"
)

func TestRemainingNonKeyProducerArgumentContract(t *testing.T) {
	for _, origin := range []float64{12.5, 13.5, 18.2} {
		timeline := remainingNonKeyContractTimeline(t, origin, false)
		base := []string{"-v", "error"}
		actual, err := copiedHLSSeekArguments(append([]string(nil), base...), timeline, 0)
		want := append(base, "-copypriorss:v", "0", "-avoid_negative_ts", "disabled",
			"-output_ts_offset", copiedHLSTime(timeline.point(0)-origin))
		if err != nil || !reflect.DeepEqual(actual, want) {
			t.Errorf("nonkey initial producer did not preserve video-only prior rule and signed presentation clock")
		}
		segments := []string{"-hls_time", "2", "-hls_segment_options", "movflags=+frag_discont+skip_sidx"}
		actual = indexedCopiedHLSSegmentArguments(append([]string(nil), segments...), timeline)
		want = []string{"-hls_time", "2", "-hls_segment_options", "movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1"}
		if !reflect.DeepEqual(actual, want) {
			t.Error("nonkey initial producer does not match the independently qualified segment geometry")
		}
	}
}

func TestRemainingNonKeyProducerArgumentDamageStaysClosed(t *testing.T) {
	for _, damage := range []string{"refill", "negative", "proof", "clock", "missing", "decode", "strategy", "exact"} {
		timeline := remainingNonKeyContractTimeline(t, 12.5, false)
		number := remainingNonKeyProducerDamage(timeline, damage)
		if actual, err := copiedHLSSeekArguments([]string{"-v", "error"}, timeline, number); err == nil || actual != nil {
			t.Errorf("damaged pending producer acquired timestamp arguments: %s", damage)
		}
	}
}

func remainingNonKeyProducerDamage(timeline *copiedHLSTimeline, damage string) int {
	number := 0
	switch damage {
	case "refill":
		number = 1
	case "negative":
		number = -1
	case "proof":
		timeline.Presentation.Proof = &copiedHLSPresentationProof{}
	case "clock":
		clock := 0.0
		timeline.Clock = &clock
	case "missing":
		timeline.Presentation = nil
	case "decode":
		timeline.Presentation.Decode.PTS++
	case "strategy":
		timeline.Strategy = "h264-idr-preroll-unknown"
	case "exact":
		timeline.Presentation.RequestedMicros = 12_000_000
	}
	return number
}

func TestRemainingNonKeyProducerLeavesLegacyAndUnindexedArgumentsExact(t *testing.T) {
	base := []string{"-v", "error"}
	actual, err := copiedHLSSeekArguments(append([]string(nil), base...), nil, 0)
	if err != nil || !reflect.DeepEqual(actual, base) {
		t.Fatal("nonkey work changed unindexed producer")
	}
	legacy := &copiedHLSTimeline{Strategy: "h264-idr-keys-1", TimeBase: 0.001,
		Keys: []copiedHLSKey{{PTS: 12000}, {PTS: 14000}}}
	clock := 0.083
	legacy.Clock = &clock
	actual, err = copiedHLSSeekArguments(append([]string(nil), base...), legacy, 1)
	want := append(base, "-copypriorss", "0", "-output_ts_offset", "2.083000",
		"-bsf:a", "setts=pts=PTS-0.083000/TB:dts=DTS-0.083000/TB")
	if err != nil || !reflect.DeepEqual(actual, want) {
		t.Fatal("nonkey work changed the certified legacy refill rule")
	}
	segments := []string{"-hls_time", "2", "-hls_segment_options", "movflags=+frag_discont+skip_sidx"}
	want = []string{"-hls_time", "0.1", "-hls_segment_options", "movflags=+frag_discont+skip_sidx"}
	if !reflect.DeepEqual(indexedCopiedHLSSegmentArguments(append([]string(nil), segments...), legacy), want) ||
		!reflect.DeepEqual(indexedCopiedHLSSegmentArguments(append([]string(nil), segments...), nil), segments) {
		t.Fatal("nonkey work changed legacy or unindexed segment policy")
	}
}

func TestRemainingNonKeyProducerRecipeScope(t *testing.T) {
	pending := remainingNonKeyContractTimeline(t, 12.5, false)
	if !copiedHLSPendingProducerRecipe(pending, hlsRecipe{mode: "remux"}) {
		t.Fatal("pending H264 remux private producer rejected")
	}
	for _, mode := range []string{"audio-transcode", "transcode", "direct", ""} {
		if copiedHLSPendingProducerRecipe(pending, hlsRecipe{mode: mode}) {
			t.Errorf("pending producer acquired unsupported source recipe: %s", mode)
		}
	}
	for _, recipe := range []hlsRecipe{{mode: "remux", dialogueBoost: true}, {mode: "remux", normalizeLoudness: true}, {mode: "remux", omitted: []PlaybackRange{{Start: 0, End: 1}}}} {
		if copiedHLSPendingProducerRecipe(pending, recipe) {
			t.Fatal("pending producer accepted changed source geometry")
		}
	}
	if !copiedHLSPendingProducerRecipe(nil, hlsRecipe{mode: "transcode"}) {
		t.Fatal("pending producer scope changed ordinary transcode")
	}
}

func TestRemainingNonKeyProducerWindowScope(t *testing.T) {
	pending := remainingNonKeyContractTimeline(t, 12.5, false)
	source := hlsRecipe{mode: "remux", offset: 12.5}
	if !copiedHLSPendingProducerWindow(pending, source, hlsRecipe{mode: "remux"}) {
		t.Fatal("pending producer rejected its resolved Remux window")
	}
	for _, mode := range []string{"audio-transcode", "transcode", "direct", ""} {
		if copiedHLSPendingProducerWindow(pending, source, hlsRecipe{mode: mode}) {
			t.Errorf("pending producer copied clock acquired mismatched resolved window: %s", mode)
		}
	}
	if !copiedHLSPendingProducerWindow(nil, hlsRecipe{mode: "transcode"}, hlsRecipe{mode: "audio-transcode"}) {
		t.Fatal("pending producer window guard changed an ordinary producer")
	}
}
