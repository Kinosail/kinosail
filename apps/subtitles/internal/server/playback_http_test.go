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

	tracks := playbackSubtitles(item, media, provider, "en", true)
	if len(tracks) != 2 || tracks[0].Default || !tracks[1].Default {
		t.Fatalf("enabled tracks = %#v", tracks)
	}
	tracks = playbackSubtitles(item, media, provider, "en", false)
	if tracks[0].Default || tracks[1].Default {
		t.Fatalf("disabled tracks = %#v", tracks)
	}
}
