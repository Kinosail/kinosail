package server

import (
	"reflect"
	"strings"
	"testing"
)

// Gap: the public AAC-tail proof detects loss but cannot distinguish a scoped
// video preroll rule from one that silently suppresses copied audio too.
func TestCopiedAACProducerVideoPrerollKeepsAudio(t *testing.T) {
	clock := 0.0
	timeline := &copiedHLSTimeline{
		Policy: "test:copied-aac=2", Strategy: "h264-idr-keys-1", End: 15,
		Numerator: 1, Denominator: 16000, TimeBase: 1.0 / 16000,
		Keys:        []copiedHLSKey{{PTS: 0, DTS: -1333}, {PTS: 48000, DTS: 46667}, {PTS: 96000, DTS: 94667}, {PTS: 144000, DTS: 142667}, {PTS: 192000, DTS: 190667}},
		Clock:       &clock,
		AudioOrigin: &copiedHLSAudioOrigin{SourceTrack: 1, Physical: 0, Edit: 0, FirstHash: "SHA256:" + strings.Repeat("a", 64)},
	}
	for _, number := range []int{0, 4} {
		arguments, err := copiedHLSProducerArguments([]string{"-c:v", "copy", "-c:a", "copy"}, timeline, number)
		if err != nil {
			t.Fatal(err)
		}
		assertCopiedAACVideoPreroll(t, arguments, number)
	}
}

func TestCopiedAACProducerKeepsLegacyAndAdmission(t *testing.T) {
	original := []string{"-c:v", "copy", "-c:a", "copy"}
	arguments, err := copiedHLSProducerArguments(append([]string(nil), original...), nil, 0)
	if err != nil || !reflect.DeepEqual(arguments, original) {
		t.Fatal("unindexed legacy arguments changed")
	}
	timeline := &copiedHLSTimeline{Keys: []copiedHLSKey{{PTS: 0}, {PTS: 16000}}, TimeBase: 1.0 / 16000}
	for _, number := range []int{-1, 1, 2} {
		if _, err := copiedHLSProducerArguments(append([]string(nil), original...), timeline, number); err == nil {
			t.Fatalf("invalid or unbound segment %d was admitted", number)
		}
	}
}

func assertCopiedAACVideoPreroll(t *testing.T, arguments []string, number int) {
	t.Helper()
	videoRule := 0
	for index, option := range arguments {
		if option == "-copypriorss" || option == "-copypriorss:a" {
			t.Fatalf("segment %d suppresses copied audio preroll: %q", number, option)
		}
		if option == "-copypriorss:v" {
			if index+1 >= len(arguments) || arguments[index+1] != "0" {
				t.Fatalf("segment %d lost the video preroll rejection", number)
			}
			videoRule++
		}
	}
	if videoRule != 1 {
		t.Fatalf("segment %d has %d video-only rules", number, videoRule)
	}
}
