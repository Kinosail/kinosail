package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestPlaybackSubtitlesOwnsDefaultSelection(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "film", Path: "Film.mkv", Subtitles: []string{"Film.en.srt", "Film.es.srt"}}
	media := probeResult{SubtitleFacts: []SubtitleFacts{{SourceIndex: 2, Language: "fra", Text: true, Default: true}}}

	tracks := playbackSubtitles(item, media, "eng", true)
	assertPreferredSubtitleDefaults(t, tracks)
	tracks = playbackSubtitles(item, media, "de", true)
	if !tracks[0].Default || tracks[1].Default || tracks[2].Default {
		t.Fatalf("missing language fallback = %#v", tracks)
	}
	tracks = playbackSubtitles(item, media, "eng", false)
	if tracks[0].Default || tracks[1].Default {
		t.Fatalf("disabled tracks = %#v", tracks)
	}
}

func TestPlaybackSubtitlesUseReadableAndVerifiedLabels(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "film", Path: "Film.mkv", Subtitles: []string{"Film.en.srt", "Film.en.FORCED.srt", "Film.fr.sdh.srt", "Film.hi.srt", "Film.en.hi.srt"}}
	media := probeResult{SubtitleFacts: []SubtitleFacts{
		{SourceIndex: 2, Language: "eng", Role: "translation", Text: true},
		{SourceIndex: 3, Language: "eng", Role: "translation", Text: true, Forced: true},
		{SourceIndex: 4, Language: "fra", Role: "captions", Text: true},
		{SourceIndex: 5, Language: "invalid<language>", Role: "translation", Text: true},
	}}
	tracks := playbackSubtitles(item, media, "en", true)
	want := []string{"English · Subtitles", "English · Forced", "French · Captions", "Subtitles", "English · Subtitles", "English · Forced", "French · Captions", "Hindi · Subtitles", "English · Captions"}
	if len(tracks) != len(want) {
		t.Fatalf("tracks = %#v", tracks)
	}
	for index, track := range tracks {
		if track.Label != want[index] {
			t.Errorf("track %d label = %q, want %q", index, track.Label, want[index])
		}
	}
	if tracks[0].Role != "" || tracks[1].Role != "" {
		t.Fatalf("inferred translation leaked as a verified role: %#v", tracks[:2])
	}
	if tracks[4].Language != "en" || tracks[5].Language != "en" || !tracks[5].Forced || tracks[6].Kind != "captions" || tracks[6].Role != "captions" || tracks[7].Kind != "subtitles" || tracks[8].Kind != "captions" {
		t.Fatalf("sidecar playback metadata changed: %#v", tracks[4:])
	}
}

func assertPreferredSubtitleDefaults(t *testing.T, tracks []subtitleTrack) {
	t.Helper()
	if len(tracks) != 3 || tracks[0].Default || !tracks[1].Default || tracks[1].Language != "en" || tracks[2].Default {
		t.Fatalf("preferred language defaults = %#v", tracks)
	}
}

func TestAppleWebKitReportsNativeSurroundAudio(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1")
	capabilities := browserPlaybackCapabilitiesForRequest(request, &settingsStore{}, nil)
	if !playback.Includes(capabilities.AudioCodecs, "ac3") || !playback.Includes(capabilities.AudioCodecs, "eac3") {
		t.Fatalf("Apple audio codecs = %#v", capabilities.AudioCodecs)
	}
}

func TestPlaybackSubtitlesPreferRegularOverForced(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		media    probeResult
		sidecars []string
		language string
		enabled  bool
		want     int
	}{
		{"embedded forced first", probeResult{SubtitleFacts: []SubtitleFacts{{Language: "eng", Text: true, Forced: true, Default: true}, {Language: "eng", Text: true}}}, nil, "en", true, 1},
		{"sidecar forced first", probeResult{}, []string{"Film.en.forced.srt", "Film.en.srt"}, "en", true, 1},
		{"uppercase forced", probeResult{}, []string{"Film.en.FORCED.srt", "Film.en.srt"}, "en", true, 1},
		{"regular fallback", probeResult{SubtitleFacts: []SubtitleFacts{{Language: "eng", Text: true, Forced: true, Default: true}, {Language: "fr", Text: true}}}, nil, "en", true, 1},
		{"only forced", probeResult{}, []string{"Film.en.forced.srt"}, "en", true, -1},
		{"off", probeResult{}, []string{"Film.en.forced.srt", "Film.en.srt"}, "en", false, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracks := playbackSubtitles(library.Item{ID: "film", Path: "Film.mkv", Subtitles: test.sidecars}, test.media, test.language, test.enabled)
			if len(tracks) != len(test.media.SubtitleFacts)+len(test.sidecars) {
				t.Fatalf("tracks removed: %#v", tracks)
			}
			for index, track := range tracks {
				if track.Default != (index == test.want) {
					t.Fatalf("track %d default = %v, wanted selected %d", index, track.Default, test.want)
				}
			}
		})
	}
}
