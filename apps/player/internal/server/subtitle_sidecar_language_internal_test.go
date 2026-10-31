package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestPlayerReadsRegionalAndNamedSidecarLanguagesLikeSubtitlesWritesThem(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "item", Kind: "video", Path: "/media/Film.mkv", Subtitles: []string{
		"/media/Film.pt-BR.srt", "/media/Film.English.srt", "/media/Film.Spanish.forced.srt", "/media/Film.zh-Hant.sdh.srt", "/media/Film.en.srt",
	}}
	want := []string{"pt", "en", "es", "zh", "en"}
	candidates := subtitleCandidates(item, probeResult{})
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %d", len(candidates))
	}
	for index, candidate := range candidates {
		if candidate.option.Language != want[index] {
			t.Errorf("%s: language = %q, want %q", item.Subtitles[index], candidate.option.Language, want[index])
		}
	}
	for subtitle, language := range map[string]string{
		"/media/Film.pt-BR.srt": "pt-BR", "/media/Film.English.srt": "en", "/media/Film.en.sdh.srt": "en", "/media/Film.srt": "und", "/media/Film.Directors.srt": "und",
	} {
		if got := subtitleLanguageFromPath("/media/Film.mkv", subtitle); got != language {
			t.Errorf("track language for %s = %q, want %q", subtitle, got, language)
		}
	}
}
