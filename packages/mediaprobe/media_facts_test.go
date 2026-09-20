package mediaprobe

import (
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestMediaFactsPreservesTracksAndAddsExternalSubtitleIdentity(t *testing.T) {
	result := Result{AudioFacts: []AudioFacts{{Index: 2}}, SubtitleFacts: []SubtitleFacts{{Index: 0}}, RandomAccess: []float64{1, 3}, Duration: 8, Bitrate: 1000}
	item := library.Item{Kind: "video", Path: "/media/film.mkv", Container: "MKV", Subtitles: []string{"/media/film.en.SRT"}}
	calls := 0
	facts := result.MediaFacts(item, "version", func(media, subtitle string) string {
		calls++
		if media != item.Path || subtitle != item.Subtitles[0] {
			t.Fatal("subtitle identity lost")
		}
		return "en"
	}, func(path string) string {
		if path != item.Subtitles[0] {
			t.Fatal("subtitle role path lost")
		}
		return "standard"
	})

	expected := playback.MediaFacts{Kind: "video", FileVersion: "version", Container: "mkv", Seekable: true, Duration: 8, Bitrate: 1000, Audio: []AudioFacts{{Index: 2}}, Subtitles: []SubtitleFacts{{Index: 0}, {Index: 1, SourceIndex: -1, ExternalIndex: 0, External: true, Text: true, Codec: "srt", Language: "en", Role: "standard"}}, RandomAccess: []float64{1, 3}}
	if calls != 1 || !reflect.DeepEqual(facts, expected) {
		t.Fatalf("facts = %#v", facts)
	}

	facts.Audio[0].Index = 9
	facts.Subtitles[0].Index = 9
	facts.RandomAccess[0] = 9
	if got := []float64{float64(result.AudioFacts[0].Index), float64(result.SubtitleFacts[0].Index), result.RandomAccess[0]}; !reflect.DeepEqual(got, []float64{2, 0, 1}) {
		t.Fatal("facts mutated cached probe slices")
	}
	result.Container = "mp4"
	if got := result.MediaFacts(library.Item{}, "", nil, nil); got.Container != "mp4" {
		t.Fatalf("probed container = %q", got.Container)
	}
}
