package servertest

import (
	"math"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

type PlaybackPlanFixture struct{ Policy playback.DecisionPolicy }

func (fixture PlaybackPlanFixture) decidePlayback(facts playback.MediaFacts, client playback.ClientCapabilities, policy playback.ViewerPolicy, intent playback.NetworkIntent) playback.PlaybackPlan {
	return playback.Decide(facts, client, policy, intent, fixture.Policy)
}

func (fixture PlaybackPlanFixture) DecidePlaybackGoldenPlans(t *testing.T) {
	t.Helper()
	t.Parallel()
	index := func(value int) *int { return &value }
	base := playback.MediaFacts{
		Kind: "video", Container: "mp4", Bitrate: 8_000_000,
		Video: playback.VideoFacts{Codec: "h264", Width: 1920, Height: 1080},
		Audio: []playback.AudioFacts{{Index: 1, Codec: "aac", Language: "en", Default: true}},
		Subtitles: []playback.SubtitleFacts{
			{Index: 2, Codec: "webvtt", Language: "en", Role: "captions", Text: true, External: true},
			{Index: 3, Codec: "pgs", Language: "ja", Role: "translation", Forced: true},
		},
	}
	web := playback.ClientCapabilities{
		Containers: []string{"mp4", "webm"}, VideoCodecs: []string{"h264", "vp9", "av1"}, AudioCodecs: []string{"aac", "opus"},
		TranscodeVideoCodecs: []string{"av1", "h264"},
		TextSubtitleCodecs:   []string{"webvtt"}, SupportsExternalSubtitles: true, SupportsRemux: true, MaxWidth: 3840, MaxHeight: 2160, HDRFormats: []string{"sdr"},
	}
	allowed := playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	tests := []struct {
		name   string
		facts  playback.MediaFacts
		client playback.ClientCapabilities
		policy playback.ViewerPolicy
		intent playback.NetworkIntent
		mode   string
		subs   string
		color  string
	}{
		{"direct", base, web, allowed, playback.NetworkIntent{SubtitleIndex: index(2)}, "direct", "external", "preserve"},
		{"direct preferred", withVideoCodec(base, "hevc"), web, allowed, playback.NetworkIntent{PreferDirect: true}, "direct", "none", "preserve"},
		{"compatibility remux", base, web, allowed, playback.NetworkIntent{PreferCompatibility: true}, "remux", "none", "preserve"},
		{"compatibility audio-only", withAudioCodec(base, "truehd"), web, allowed, playback.NetworkIntent{PreferCompatibility: true}, "audio-transcode", "none", "preserve"},
		{"remux", withContainer(base, "mkv"), web, allowed, playback.NetworkIntent{}, "remux", "none", "preserve"},
		{"audio-only", withAudioCodec(base, "truehd"), web, allowed, playback.NetworkIntent{}, "audio-transcode", "none", "preserve"},
		{"video", withVideoCodec(base, "hevc"), web, allowed, playback.NetworkIntent{}, "transcode", "none", "preserve"},
		{"hdr", withHDR(base, "hdr10"), web, allowed, playback.NetworkIntent{}, "transcode", "none", "tone-map-sdr"},
		{"image subtitle", base, web, allowed, playback.NetworkIntent{SubtitleIndex: index(3)}, "transcode", "burn-in", "preserve"},
		{"bitrate", base, web, allowed, playback.NetworkIntent{MaxBitrate: 4_000_000}, "transcode", "none", "preserve"},
		{"policy", base, web, playback.ViewerPolicy{}, playback.NetworkIntent{}, "denied", "none", "preserve"},
		{"no transcode permission", withVideoCodec(base, "hevc"), web, playback.ViewerPolicy{AllowPlayback: true}, playback.NetworkIntent{}, "denied", "none", "preserve"},
		{"unknown facts", playback.MediaFacts{Kind: "video", Container: "mkv"}, web, allowed, playback.NetworkIntent{}, "transcode", "none", "preserve"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := fixture.decidePlayback(test.facts, test.client, test.policy, test.intent)
			if plan.Mode != test.mode || plan.SubtitleMode != test.subs || plan.ColorMode != test.color || plan.Reason == "" || plan.Allowed != (test.mode != "denied") {
				t.Fatalf("plan = %#v", plan)
			}
			if test.mode == "transcode" && plan.VideoCodec != "av1" {
				t.Fatalf("transcode codec = %q, want av1", plan.VideoCodec)
			}
		})
	}
}

func (fixture PlaybackPlanFixture) AdaptiveQualitiesPreserveSourceShapeWithoutUpscaling(t *testing.T) {
	t.Helper()
	t.Parallel()
	plan := fixture.decidePlayback(
		playback.MediaFacts{Kind: "video", Container: "mp4", Video: playback.VideoFacts{Codec: "hevc", Width: 1080, Height: 1920, FrameRate: 30}, Audio: []playback.AudioFacts{{Codec: "aac"}}},
		playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxWidth: 3840, MaxHeight: 2160},
		playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, playback.NetworkIntent{},
	)
	if len(plan.Qualities) < 2 {
		t.Fatalf("qualities = %#v", plan.Qualities)
	}
	for _, quality := range plan.Qualities {
		if quality.Width > 1080 || quality.Height > 1920 || quality.Width%2 != 0 || quality.Height%2 != 0 || math.Abs(float64(quality.Width)/float64(quality.Height)-1080.0/1920.0) > 0.01 {
			t.Fatalf("quality distorts or upscales portrait source: %#v", quality)
		}
	}
	last := plan.Qualities[len(plan.Qualities)-1]
	if last.Width != 606 || last.Height != 1080 {
		t.Fatalf("top quality = %#v", last)
	}
}

func (fixture PlaybackPlanFixture) DirectFirstHasNoImplicitBitrateLimit(t *testing.T) {
	t.Helper()
	t.Parallel()
	facts := playback.MediaFacts{Kind: "video", Container: "mp4", Bitrate: 120_000_000, Video: playback.VideoFacts{Codec: "h264", Width: 3840, Height: 2160}, Audio: []playback.AudioFacts{{Codec: "aac"}}}
	client := playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxWidth: 3840, MaxHeight: 2160}
	policy := playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	if plan := fixture.decidePlayback(facts, client, policy, playback.NetworkIntent{PreferDirect: true}); plan.Mode != "direct" || plan.MaxBitrate != 0 {
		t.Fatalf("unlimited Direct First plan = %#v", plan)
	}
	if plan := fixture.decidePlayback(facts, client, policy, playback.NetworkIntent{MaxBitrate: 20_000_000}); plan.Mode != "transcode" || plan.Reason != "bitrate-exceeds-limit" {
		t.Fatalf("explicit bitrate limit plan = %#v", plan)
	}
}

func (fixture PlaybackPlanFixture) CompatibilityUsesSupportedAlternateAudio(t *testing.T) {
	t.Helper()
	t.Parallel()
	facts := playback.MediaFacts{Kind: "video", Container: "mkv", Video: playback.VideoFacts{Codec: "h264"}, Audio: []playback.AudioFacts{{Index: 0, Codec: "truehd", Default: true}, {Index: 1, Codec: "ac3"}}}
	client := playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac", "ac3"}, SupportsRemux: true}
	plan := fixture.decidePlayback(facts, client, playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, playback.NetworkIntent{PreferCompatibility: true})
	if plan.Mode != "remux" || plan.AudioIndex != 1 || plan.AudioCodec != "ac3" {
		t.Fatalf("alternate audio plan = %#v", plan)
	}
}

func (fixture PlaybackPlanFixture) AdaptiveQualitiesUseConventionalTierForCroppedVideo(t *testing.T) {
	t.Helper()
	t.Parallel()
	plan := fixture.decidePlayback(
		playback.MediaFacts{Kind: "video", Container: "mp4", Video: playback.VideoFacts{Codec: "hevc", Width: 1920, Height: 804}, Audio: []playback.AudioFacts{{Codec: "aac"}}},
		playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxWidth: 1920, MaxHeight: 1080},
		playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, playback.NetworkIntent{},
	)
	want := []string{"360p", "432p", "540p", "720p", "1080p"}
	if len(plan.Qualities) != len(want) {
		t.Fatalf("qualities = %#v", plan.Qualities)
	}
	for index, quality := range plan.Qualities {
		if quality.Label != want[index] {
			t.Fatalf("qualities = %#v", plan.Qualities)
		}
	}
	last := plan.Qualities[len(plan.Qualities)-1]
	if last.Label != "1080p" || last.Width != 1920 || last.Height != 804 {
		t.Fatalf("top quality = %#v", last)
	}
}

func (fixture PlaybackPlanFixture) AdaptiveQualitiesMatchFFmpegEvenRounding(t *testing.T) {
	t.Helper()
	t.Parallel()
	plan := fixture.decidePlayback(
		playback.MediaFacts{Kind: "video", Container: "mkv", Video: playback.VideoFacts{Codec: "hevc", Width: 1920, Height: 960}, Audio: []playback.AudioFacts{{Codec: "aac"}}},
		playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxWidth: 1920, MaxHeight: 1080},
		playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, playback.NetworkIntent{},
	)
	want := [][2]int{{678, 340}, {814, 408}, {1018, 510}, {1358, 680}, {1920, 960}}
	if len(plan.Qualities) != len(want) {
		t.Fatalf("qualities = %#v", plan.Qualities)
	}
	for index, dimensions := range want {
		if plan.Qualities[index].Width != dimensions[0] || plan.Qualities[index].Height != dimensions[1] {
			t.Fatalf("quality must match FFmpeg scale -2 output: %#v", plan.Qualities)
		}
	}
}

func withContainer(facts playback.MediaFacts, container string) playback.MediaFacts {
	facts.Container = container
	return facts
}

func withAudioCodec(facts playback.MediaFacts, codec string) playback.MediaFacts {
	facts.Audio = append([]playback.AudioFacts(nil), facts.Audio...)
	facts.Audio[0].Codec = codec
	return facts
}

func withVideoCodec(facts playback.MediaFacts, codec string) playback.MediaFacts {
	facts.Video.Codec = codec
	return facts
}

func withHDR(facts playback.MediaFacts, format string) playback.MediaFacts {
	facts.Video.HDR = format
	facts.Video.BitDepth = 10
	return facts
}
