package server

import "testing"

func TestDirectTypeOnlyClaimsCodecParametersSupportedByProbeFacts(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		path  string
		facts MediaFacts
		want  string
	}{
		"H264 AAC MP4":  {"film.mp4", MediaFacts{Video: VideoFacts{Codec: "h264", Profile: "High", Level: "40"}, Audio: []AudioFacts{{Codec: "aac", Profile: "LC"}}}, `video/mp4; codecs="avc1.640028, mp4a.40.2"`},
		"H264 AC3 MP4":  {"film.mp4", MediaFacts{Video: VideoFacts{Codec: "h264", Profile: "High", Level: "40"}, Audio: []AudioFacts{{Codec: "ac3"}}}, `video/mp4; codecs="avc1.640028, ac-3"`},
		"VP9 Opus WebM": {"film.webm", MediaFacts{Video: VideoFacts{Codec: "vp9"}, Audio: []AudioFacts{{Codec: "opus"}}}, `video/webm; codecs="vp9, opus"`},
		"AV1 Opus WebM": {"film.webm", MediaFacts{Video: VideoFacts{Codec: "av1"}, Audio: []AudioFacts{{Codec: "opus"}}}, `video/webm; codecs="av01, opus"`},
		"AV1 AAC MP4":   {"film.mp4", MediaFacts{Video: VideoFacts{Codec: "av1"}, Audio: []AudioFacts{{Codec: "aac", Profile: "LC"}}}, `video/mp4; codecs="av01, mp4a.40.2"`},
		"Matroska":      {"film.mkv", MediaFacts{}, "video/x-matroska"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := directMediaType(test.path, test.facts); got != test.want {
				t.Fatalf("directMediaType() = %q, want %q", got, test.want)
			}
		})
	}
	unknown := MediaFacts{Video: VideoFacts{Codec: "hevc"}, Audio: []AudioFacts{{Codec: "eac3"}}}
	if got := directMediaType("film.mp4", unknown); got != "" {
		t.Fatalf("unknown type = %q", got)
	}
	unsupportedAudio := MediaFacts{Video: VideoFacts{Codec: "h264", Profile: "High", Level: "40"}, Audio: []AudioFacts{{Codec: "truehd"}}}
	if got := directMediaType("film.mp4", unsupportedAudio); got != "" {
		t.Fatalf("unsupported audio type = %q", got)
	}
}
