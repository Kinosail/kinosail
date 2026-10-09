package server

import (
	"reflect"
	"strings"
	"testing"
)

// Gap: the public AAC-tail proof detects loss but cannot distinguish a scoped
// video preroll rule from one that silently suppresses copied audio too.
func TestCopiedHLSVideoPrerollDoesNotSuppressCopiedAudio(t *testing.T) {
	clock := 0.0
	timeline := &copiedHLSTimeline{
		Numerator: 1, Denominator: 16000, TimeBase: 1.0 / 16000,
		Keys:     []copiedHLSKey{{PTS: 0}, {PTS: 48000}, {PTS: 96000}, {PTS: 144000}, {PTS: 192000}},
		Clock:    &clock,
		AudioOrigin: &copiedHLSAudioOrigin{SourceTrack: 1, Physical: 0, Edit: 0, FirstHash: "SHA256:" + strings.Repeat("a", 64)},
	}
	for _, number := range []int{0, 4} {
		arguments, err := copiedHLSSeekArguments([]string{"-c:v", "copy", "-c:a", "copy"}, timeline, number)
		if err != nil {
			t.Fatal(err)
		}
		assertCopiedHLSVideoPreroll(t, arguments, number)
	}
}

func TestCopiedHLSPrerollScopeKeepsLegacyAndAdmission(t *testing.T) {
	original := []string{"-c:v", "copy", "-c:a", "copy"}
	arguments, err := copiedHLSSeekArguments(append([]string(nil), original...), nil, 0)
	if err != nil || !reflect.DeepEqual(arguments, original) {
		t.Fatal("unindexed legacy arguments changed")
	}
	timeline := &copiedHLSTimeline{Keys: []copiedHLSKey{{PTS: 0}, {PTS: 16000}}, TimeBase: 1.0 / 16000}
	for _, number := range []int{-1, 1, 2} {
		if _, err := copiedHLSSeekArguments(append([]string(nil), original...), timeline, number); err == nil {
			t.Fatalf("invalid or unbound segment %d was admitted", number)
		}
	}
}

func assertCopiedHLSVideoPreroll(t *testing.T, arguments []string, number int) {
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
