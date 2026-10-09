package server

import (
	"math"
	"strings"
	"testing"
)

// Gap: payload E2E catches loss/duplication but cannot distinguish DTS clipping,
// seek-floor rounding, clock cancellation, or malformed cache metadata.
func TestCopiedHLSRefillAudioKeepsDTSBoundaryAndCanonicalClock(t *testing.T) {
	for _, key := range []copiedHLSKey{{PTS: 960000, DTS: 956000}, {PTS: 960001, DTS: 956001}} {
		timeline := copiedHLSAudioBoundaryTimeline(key)
		arguments, err := copiedHLSSeekArguments([]string{"-c:v", "copy", "-c:a", "copy"}, timeline, 1)
		if err != nil {
			t.Fatal(err)
		}
		floor := "20.000000"
		offset := "8.095833"
		if key.PTS == 960001 {
			floor, offset = "20.000020", "8.095853"
		}
		boundary := "956000"
		if key.DTS == 956001 {
			boundary = "956001"
		}
		want := "noise=amount=0:drop=lt(pts+round(" + floor + "/tb)\\,ceil(" + boundary + "*1*round(1/tb)/48000))"
		if audio := copiedHLSAudioOption(arguments, "-bsf:a"); audio != want {
			t.Fatalf("DTS boundary or payload preservation changed: %q", audio)
		}
		if got := copiedHLSAudioOption(arguments, "-output_ts_offset"); got != offset {
			t.Fatalf("canonical clock was canceled: %q", got)
		}
		assertCopiedHLSVideoPreroll(t, arguments, 1)
	}
}

func TestCopiedHLSRefillAudioRejectsInvalidDTSMetadata(t *testing.T) {
	mutations := []func(*copiedHLSTimeline){
		func(v *copiedHLSTimeline) { v.Keys[1].DTS = v.Keys[1].PTS + 1 },
		func(v *copiedHLSTimeline) { v.Keys[1].DTS = v.Keys[0].DTS },
		func(v *copiedHLSTimeline) {
			v.Keys[1].PTS = v.Keys[0].PTS - 1
			v.Keys[1].DTS = v.Keys[0].DTS + 1
		},
		func(v *copiedHLSTimeline) { v.Keys[1].DTS = -(1 << 52) - 1 },
		func(v *copiedHLSTimeline) { v.Keys[1].PTS = (1 << 52) + 1 },
		func(v *copiedHLSTimeline) { v.Numerator = 0 },
		func(v *copiedHLSTimeline) { v.Denominator = (1 << 52) + 1 },
		func(v *copiedHLSTimeline) { v.TimeBase = math.NaN() },
		func(v *copiedHLSTimeline) { v.TimeBase *= 2 },
	}
	for number, mutate := range mutations {
		timeline := copiedHLSAudioBoundaryTimeline(copiedHLSKey{PTS: 960000, DTS: 956000})
		mutate(timeline)
		if arguments, err := copiedHLSSeekArguments([]string{"-c:a", "copy"}, timeline, 1); err == nil || arguments != nil {
			t.Fatalf("malformed DTS metadata %d was admitted", number)
		}
	}
}

func TestCopiedHLSRefillAudioRetainsPrefixAndEncodedAudio(t *testing.T) {
	timeline := copiedHLSAudioBoundaryTimeline(copiedHLSKey{PTS: 960000, DTS: 956000})
	prefix, err := copiedHLSSeekArguments([]string{"-c:a", "copy"}, timeline, 0)
	if err != nil || copiedHLSAudioOption(prefix, "-bsf:a") != "" || copiedHLSAudioOption(prefix, "-output_ts_offset") != "" {
		t.Fatal("prefix gained a refill rule")
	}
	encoded, err := copiedHLSSeekArguments([]string{"-c:a", "aac"}, timeline, 1)
	if err != nil || copiedHLSAudioOption(encoded, "-bsf:a") != "setts=pts=PTS-0.095833/TB:dts=DTS-0.095833/TB" {
		t.Fatal("encoded audio behavior changed")
	}
	overridden, err := copiedHLSSeekArguments([]string{"-c:a", "copy", "-c:a", "aac"}, timeline, 1)
	if err != nil || strings.HasPrefix(copiedHLSAudioOption(overridden, "-bsf:a"), "noise=") {
		t.Fatal("overridden encoded audio received copied-packet clipping")
	}
}

func copiedHLSAudioBoundaryTimeline(key copiedHLSKey) *copiedHLSTimeline {
	clock := 0.095833
	return &copiedHLSTimeline{
		Numerator:   1,
		Denominator: 48000,
		TimeBase:    1.0 / 48000,
		Keys:        []copiedHLSKey{{PTS: 576000, DTS: 572000}, key},
		Clock:       &clock,
	}
}

func copiedHLSAudioOption(arguments []string, wanted string) string {
	result := ""
	for index, option := range arguments {
		if option == wanted && index+1 < len(arguments) {
			result = arguments[index+1]
		}
	}
	return result
}
