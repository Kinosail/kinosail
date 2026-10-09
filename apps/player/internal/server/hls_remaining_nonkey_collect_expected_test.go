package server

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// Fixed expected correspondence was independently measured from complete source
// and whole native PCM controls in hosted run 37816814467, not this producer.
func remainingNonKeyCollectorExpected(t *testing.T, source string, offset float64) (string, []int64) {
	t.Helper()
	cases := map[string]struct {
		hash  string
		clock []int64
	}{
		".mkv-12.5": {"a300712f1935065b6264aaf2dfde818ccd8b19d4e27a91edb0b58084a3a8beb1", []int64{11925, 572416, 599040, 599040, 27584, 1024, 0, 1000, 27600}},
		".mkv-13.5": {"a300712f1935065b6264aaf2dfde818ccd8b19d4e27a91edb0b58084a3a8beb1", []int64{11925, 572416, 647168, 647168, 75584, 1024, 0, 1000, 75600}},
		".mkv-18.2": {"2ea9af824ab5774b3b177dad4a582b662169792671e82846fe9667b4a57f023d", []int64{17920, 860160, 873472, 873472, 13440, 1024, 0, 1000, 13440}},
		".mp4-12.5": {"be169ea0003fd2a38832778e186ca677dbb3f5a1aae09157557c800b3c4031f9", []int64{571400, 571408, 599048, 599056, 28600, 16, -8, 48000, 28600}},
		".mp4-13.5": {"be169ea0003fd2a38832778e186ca677dbb3f5a1aae09157557c800b3c4031f9", []int64{571400, 571408, 647176, 647184, 76600, 16, -8, 48000, 76600}},
		".mp4-18.2": {"6658d429f4398188f7b8e24671750e99b4b1004a8fd780e882fba8fdf7968294", []int64{859144, 859152, 873480, 873488, 14456, 16, -8, 48000, 14456}},
	}
	value, ok := cases[fmt.Sprintf("%s-%.1f", filepath.Ext(source), offset)]
	if !ok {
		t.Fatal("source-clock unknown independently measured fixture")
	}
	return value.hash, value.clock
}

func remainingNonKeyCollectorCorrespondence(t *testing.T, proof *copiedHLSAudioProof, expected []int64) {
	t.Helper()
	actual := []int64{proof.FirstPTS, proof.FirstNativeSample, proof.TargetPTS, proof.TargetNativeSample,
		proof.MediaTime, proof.LeadingSamples, proof.SourcePhase, proof.Denominator, proof.OriginalMediaTime}
	if !slices.Equal(actual, expected) || proof.TargetSamples != 1024 || proof.Numerator != 1 ||
		proof.SampleRate != 48000 || proof.Channels != 2 || proof.Codec != "aac" || proof.Profile != "LC" {
		t.Fatal("nonkey actual source-clock differs from independent complete-source correspondence")
	}
}
