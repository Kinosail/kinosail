package playback

import (
	"slices"
	"strings"
	"testing"
)

func TestDolbyVisionWithAnHDR10BaseLayerIsCopiedForHDR10Clients(t *testing.T) {
	appleTV := ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264", "hevc"}, AudioCodecs: []string{"aac", "ac3", "eac3"}, HDRFormats: []string{"sdr", "hdr10", "hlg"}, MaxAudioChannels: 6, SupportsRemux: true}
	sdrOnly := appleTV
	sdrOnly.HDRFormats = []string{"sdr"}
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	for _, test := range []struct {
		name               string
		profile, compat    int
		transfer           string
		client             ClientCapabilities
		mode, color, strip string
	}{
		{"profile 8.1 on HDR10 display", 8, 1, "smpte2084", appleTV, "remux", "preserve", "dovi_rpu=strip=1"},
		{"profile 8 PQ base on HDR10 display", 8, 0, "smpte2084", appleTV, "remux", "preserve", "dovi_rpu=strip=1"},
		{"profile 8.4 without HLG display", 8, 4, "arib-std-b67", sdrOnly, "transcode", "tone-map-sdr", ""},
		{"profile 8.1 on SDR display", 8, 1, "smpte2084", sdrOnly, "transcode", "tone-map-sdr", ""},
		{"profile 5 has no base layer", 5, 0, "smpte2084", appleTV, "denied", "tone-map-sdr", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := MediaFacts{Kind: "video", Container: "mkv", Duration: 60, Bitrate: 20_000_000,
				Video: VideoFacts{Codec: "hevc", Width: 3840, Height: 2160, BitDepth: 10, PixelFormat: "yuv420p10le", HDR: "dolby-vision", Transfer: test.transfer, DolbyVisionProfile: test.profile, DolbyVisionCompatibility: test.compat},
				Audio: []AudioFacts{{Index: 0, Codec: "eac3", Channels: 6}}}
			plan := Decide(facts, test.client, policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{})
			if plan.Allowed == (test.mode == "denied") || plan.Mode != test.mode || plan.ColorMode != test.color {
				t.Fatalf("plan = %s %s %s; want %s %s", plan.Mode, plan.ColorMode, plan.Reason, test.mode, test.color)
			}
			if plan.Mode != "remux" {
				return
			}
			for _, mode := range []string{"remux", "audio-transcode"} {
				arguments, err := HLSCodecArguments(HLSCodecInput{Arguments: []string{"-i", "movie.mkv"}, AudioRate: "128k", CopyInput: "0", DolbyVisionCodec: DolbyVisionCopyCodec(facts.Video), Recipe: HLSRecipe{Mode: mode}})
				copied := slices.Index(arguments, "copy")
				want := []string{"-c:v", "copy", "-bsf:v", test.strip, "-tag:v", "hvc1"}
				if err != nil || copied < 1 || len(arguments) < copied+5 || !slices.Equal(arguments[copied-1:copied+5], want) {
					t.Fatalf("%s arguments = %s, %v; want copied video without the Dolby Vision layer", mode, strings.Join(arguments, " "), err)
				}
			}
		})
	}
}

func TestOrdinaryCopiedVideoKeepsItsBitstream(t *testing.T) {
	for _, mode := range []string{"remux", "audio-transcode"} {
		arguments, err := HLSCodecArguments(HLSCodecInput{Arguments: []string{"-i", "movie.mkv"}, AudioRate: "128k", CopyInput: "0", Recipe: HLSRecipe{Mode: mode}})
		if err != nil || slices.Contains(arguments, "-bsf:v") {
			t.Fatalf("%s arguments = %s, %v", mode, strings.Join(arguments, " "), err)
		}
	}
}
