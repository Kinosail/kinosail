package downloads

import (
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestTrackBundlePreservesSelectionAndStyledSidecars(t *testing.T) { //nolint:cyclop // One output recipe must retain all selected track and sidecar arguments.
	facts := playback.MediaFacts{
		Audio:     []playback.AudioFacts{{Index: 0, SourceIndex: 1}, {Index: 1, SourceIndex: 2}},
		Subtitles: []playback.SubtitleFacts{{Index: 0, SourceIndex: 3, Text: true, Codec: "subrip"}, {Index: 1, External: true, ExternalIndex: 0, Text: true, Codec: "ass"}},
	}
	item := library.Item{Subtitles: []string{"/library/film.en.ass"}}
	inputs, maps, mkv, err := resolveTracks(facts, item, nil)
	if err != nil || !mkv || !reflect.DeepEqual(inputs, []string{"-i", "/library/film.en.ass"}) || !reflect.DeepEqual(maps, []string{"-map", "0:v:0", "-map", "0:1", "-map", "0:2", "-map", "0:3", "-map", "1:0"}) {
		t.Fatalf("bundle = %v %v %v %v", inputs, maps, mkv, err)
	}
	inputs, maps, mkv, err = resolveTracks(facts, item, &TrackSelection{Audio: []int{1}, Subtitles: []int{}})
	if err != nil || mkv || len(inputs) != 0 || !reflect.DeepEqual(maps, []string{"-map", "0:v:0", "-map", "0:2"}) {
		t.Fatalf("selection = %v %v %v %v", inputs, maps, mkv, err)
	}
	for _, selection := range []*TrackSelection{{Audio: []int{}, Subtitles: []int{9}}, {Audio: []int{9}, Subtitles: []int{}}, {Audio: []int{0, 0}, Subtitles: []int{}}, {Audio: nil, Subtitles: []int{}}} {
		if _, _, _, err := resolveTracks(facts, item, selection); err == nil {
			t.Fatalf("invalid selection accepted: %+v", selection)
		}
	}
}
