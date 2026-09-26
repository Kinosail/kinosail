package playback

import (
	"reflect"
	"testing"
)

func TestDecidePreservesPlayerPolicyAndProductAudioChoice(t *testing.T) {
	t.Parallel()
	base := MediaFacts{Kind: "video", Container: "mkv", Bitrate: 8_000_000, Video: VideoFacts{Codec: "hevc", Width: 3840, Height: 2160, FrameRate: 24}, Audio: []AudioFacts{{Index: 0, Codec: "dts", Default: true}, {Index: 1, Codec: "aac"}}}
	client := BrowserCapabilities()
	client.VideoCodecs = append(client.VideoCodecs, "hevc")
	client.MaxWidth, client.MaxHeight, client.MaxBitrate = 1920, 1080, 4_000_000
	client.TranscodeVideoCodecs = []string{"hevc", "h264"}
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}

	plan := Decide(base, client, policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{PreferCompatibleAudio: true})
	if !plan.Allowed || plan.Mode != "transcode" || plan.VideoCodec != "hevc" || plan.AudioIndex != 1 || plan.Reason != "resolution-exceeds-client" || !plan.Adaptive || len(plan.Qualities) == 0 {
		t.Fatalf("Player plan = %#v", plan)
	}
	subtitlesPlan := Decide(base, client, policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{})
	if subtitlesPlan.AudioIndex != 0 {
		t.Fatalf("product audio policy was lost: %#v", subtitlesPlan)
	}
}

func TestDecideCoversOrderedDeliveryModes(t *testing.T) {
	t.Parallel()
	index := 2
	facts := MediaFacts{Kind: "video", Container: "mkv", Video: VideoFacts{Codec: "h264", Width: 1280, Height: 720}, Audio: []AudioFacts{{Index: 0, Codec: "aac", Default: true}}, Subtitles: []SubtitleFacts{{Index: 2, SourceIndex: 7, Codec: "vtt", Text: true, External: true}}}
	client := BrowserCapabilities()
	allowed := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	tests := []struct {
		name   string
		facts  MediaFacts
		client ClientCapabilities
		policy ViewerPolicy
		intent NetworkIntent
		mode   string
		reason string
	}{
		{"playback denied", facts, client, ViewerPolicy{}, NetworkIntent{}, "denied", "playback-not-allowed"},
		{"direct request", facts, client, allowed, NetworkIntent{ForceDirect: true}, "direct", "direct-requested"},
		{"direct preferred", facts, client, allowed, NetworkIntent{PreferDirect: true}, "direct", "direct-preferred"},
		{"compatibility remux", facts, client, allowed, NetworkIntent{PreferCompatibility: true}, "remux", "compatibility-requested"},
		{"unsupported audio", withAudio(facts, "flac"), client, allowed, NetworkIntent{}, "audio-transcode", "audio-codec-unsupported"},
		{"unsupported container", facts, withoutRemux(client), allowed, NetworkIntent{}, "transcode", "container-unsupported"},
		{"forced transcode", facts, client, allowed, NetworkIntent{ForceTranscode: true}, "transcode", "transcode-requested"},
		{"subtitle burn in", withContainer(facts, "mp4"), client, allowed, NetworkIntent{SubtitleIndex: &index}, "direct", "compatible"},
		{"transcode denied", facts, client, ViewerPolicy{AllowPlayback: true}, NetworkIntent{ForceTranscode: true}, "denied", "transcoding-not-allowed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := Decide(test.facts, test.client, test.policy, test.intent, DecisionPolicy{PreferCompatibleAudio: true})
			if plan.Mode != test.mode || plan.Reason != test.reason {
				t.Fatalf("plan = %#v", plan)
			}
			if test.name == "subtitle burn in" && (plan.SubtitleMode != "external" || plan.SubtitleSourceIndex != 7) {
				t.Fatalf("subtitle plan = %#v", plan)
			}
		})
	}
}

func TestBrowserDeliversConvertibleTextSubtitlesWithoutVideoTranscoding(t *testing.T) {
	t.Parallel()
	client := BrowserCapabilities()
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	index := 0
	for _, track := range []SubtitleFacts{
		{Index: 0, SourceIndex: 2, Codec: "subrip", Text: true},
		{Index: 0, SourceIndex: 2, Codec: "ass", Text: true},
		{Index: 0, SourceIndex: -1, ExternalIndex: 0, Codec: "srt", Text: true, External: true},
	} {
		facts := MediaFacts{Kind: "video", Container: "mp4", Video: VideoFacts{Codec: "h264", Width: 1280, Height: 720}, Audio: []AudioFacts{{Index: 0, Codec: "aac", Default: true}}, Subtitles: []SubtitleFacts{track}}
		plan := Decide(facts, client, policy, NetworkIntent{SubtitleIndex: &index}, DecisionPolicy{})
		if !plan.Allowed || plan.Mode != "direct" || plan.SubtitleMode == "burn-in" {
			t.Errorf("browser text subtitle %q required video conversion: %#v", track.Codec, plan)
		}
	}
}

func TestDecideExplainsEachVideoConstraint(t *testing.T) {
	t.Parallel()
	base := MediaFacts{Kind: "video", Container: "mp4", Video: VideoFacts{Codec: "h264", Width: 1280, Height: 720, HDR: "sdr"}}
	baseClient := BrowserCapabilities()
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	tests := []struct {
		name string
		edit func(*MediaFacts, *ClientCapabilities, *NetworkIntent)
		want string
	}{
		{"codec", func(f *MediaFacts, _ *ClientCapabilities, _ *NetworkIntent) { f.Video.Codec = "mpeg2" }, "video-codec-unsupported"},
		{"resolution", func(f *MediaFacts, c *ClientCapabilities, _ *NetworkIntent) { c.MaxWidth = 640 }, "resolution-exceeds-client"},
		{"bitrate", func(f *MediaFacts, _ *ClientCapabilities, i *NetworkIntent) {
			f.Bitrate, i.MaxBitrate = 9_000_000, 1_000_000
		}, "bitrate-exceeds-limit"},
		{"hdr", func(f *MediaFacts, _ *ClientCapabilities, _ *NetworkIntent) { f.Video.HDR = "hdr10" }, "hdr-unsupported"},
		{"burn", func(f *MediaFacts, _ *ClientCapabilities, i *NetworkIntent) {
			n := 3
			i.SubtitleIndex = &n
			f.Subtitles = []SubtitleFacts{{Index: 3, Codec: "pgs"}}
		}, "subtitle-burn-in-required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts, client, intent := base, baseClient, NetworkIntent{}
			test.edit(&facts, &client, &intent)
			plan := Decide(facts, client, policy, intent, DecisionPolicy{})
			if plan.Mode != "transcode" || plan.Reason != test.want {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestPlaybackHelpersReturnCanonicalValues(t *testing.T) { //nolint:cyclop // The canonical helper matrix is intentionally checked together.
	t.Parallel()
	if NormalizeContainer(" MATROSKA ") != "mkv" || NormalizeContainer("mov,mp4,m4a,3gp,3g2,mj2") != "mp4" || NormalizeContainer("ogg") != "ogg" {
		t.Fatal("container normalization changed")
	}
	if !Includes([]string{"AAC"}, "aac") || !Includes([]string{"*"}, "anything") || Includes([]string{"aac"}, "opus") {
		t.Fatal("case-insensitive capability matching changed")
	}
	if MinimumPositive(0, 20, -1, 10) != 10 || MinimumPositiveInt(0, 20, -1, 10) != 10 {
		t.Fatal("positive minimum changed")
	}
	if got := AdaptiveQualities(640, 360, 23.976, 0); len(got) != 1 || got[0].Label != "360p" || got[0].Width != 640 || got[0].Height != 360 {
		t.Fatalf("small ladder = %#v", got)
	}
	if got := AdaptiveQualities(1920, 800, 24, 100_000); len(got) != 1 || got[0].Bitrate != 100_000 || got[0].Width%2 != 0 || got[0].Height%2 != 0 {
		t.Fatalf("capped ladder = %#v", got)
	}
	if got := AdaptiveQualities(0, 0, 0, 0); len(got) != 5 || QualityLabel(100, 50) != "50p" {
		t.Fatalf("default ladder = %#v", got)
	}
}

func TestTimelineMapsBothDirections(t *testing.T) {
	t.Parallel()
	timeline := Timeline{SourceDuration: 100, Duration: 75, Omitted: []Range{{Start: 10, End: 25}, {Start: 90, End: 100}}}
	for _, test := range []struct{ presentation, source float64 }{{0, 0}, {9, 9}, {10, 25}, {74, 89}, {75, 100}} {
		if got := timeline.SourceTime(test.presentation); got != test.source || timeline.PresentationTime(test.source) != test.presentation {
			t.Fatalf("mapping %v <-> %v produced %v <-> %v", test.presentation, test.source, got, timeline.PresentationTime(test.source))
		}
	}
	if !reflect.DeepEqual(BrowserCapabilities().Containers, []string{"mp4", "mov", "webm"}) {
		t.Fatal("browser contract changed")
	}
}

func withAudio(facts MediaFacts, codec string) MediaFacts {
	facts.Audio = []AudioFacts{{Index: 0, Codec: codec, Default: true}}
	return facts
}

func withoutRemux(client ClientCapabilities) ClientCapabilities {
	client.SupportsRemux = false
	return client
}

func withContainer(facts MediaFacts, container string) MediaFacts {
	facts.Container = container
	return facts
}
