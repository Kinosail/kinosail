package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestPlaybackSubtitlesOwnsDefaultSelection(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "film", Path: "Film.mkv", Subtitles: []string{"Film.en.srt"}}
	media := probeResult{SubtitleFacts: []SubtitleFacts{{SourceIndex: 2, Language: "fr", Text: true}}}
	provider := &subtitleProvider{cache: t.TempDir()}

	tracks := playbackSubtitles(item, media, provider, []string{"en", "fr"}, "standard", false, true)
	if len(tracks) != 2 || tracks[0].Default || !tracks[1].Default {
		t.Fatalf("enabled tracks = %#v", tracks)
	}
	tracks = playbackSubtitles(item, media, provider, []string{"en", "fr"}, "standard", false, false)
	if tracks[0].Default || tracks[1].Default {
		t.Fatalf("disabled tracks = %#v", tracks)
	}
}

func TestPlaybackSubtitlesChoosesOnePreferredTrackPerLanguage(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "film", Path: "Film.mkv", Subtitles: []string{"Film.en.srt", "Film.en.sdh.srt"}}
	media := probeResult{SubtitleFacts: []SubtitleFacts{
		{SourceIndex: 2, Language: "eng", Text: true, Forced: true, Role: "forced"},
		{SourceIndex: 3, Language: "eng", Text: true, Role: "translation"},
		{SourceIndex: 4, Language: "eng", Text: true, Role: "captions"},
	}}
	provider := &subtitleProvider{cache: t.TempDir()}
	for _, test := range []struct {
		preference string
		source     string
	}{
		{"standard", "/subtitle/film/embedded/3"},
		{"sdh", "/subtitle/film/embedded/4"},
	} {
		tracks := playbackSubtitles(item, media, provider, []string{"en"}, test.preference, true, true)
		if len(tracks) != 1 || tracks[0].Source != test.source || !tracks[0].Default {
			t.Errorf("%s tracks = %#v", test.preference, tracks)
		}
	}
}
