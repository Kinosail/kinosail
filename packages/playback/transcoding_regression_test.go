package playback

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestRecipePreservesDimensionsAndExternalSubtitleIdentity(t *testing.T) {
	plan := PlaybackPlan{Mode: "transcode", VideoCodec: "hevc", Width: 960, Height: 540, SubtitleMode: "burn-in", SubtitleText: true, SubtitleExternal: true, SubtitleExternalIndex: 2, SubtitleSourceIndex: -1}
	recipe := RecipeFor(plan)
	actual, err := ParseHLSRecipe(recipe.Token(), HLSRecipePolicy{MaxBitrate: 100000000, OffsetStepMilliseconds: 100})
	if err != nil || !reflect.DeepEqual(actual, recipe) {
		t.Fatalf("round trip = %#v, %v; want %#v", actual, err, recipe)
	}
	other := recipe
	other.Height = 480
	if HLSRecipeKey("item", other) == HLSRecipeKey("item", recipe) {
		t.Fatal("different output dimensions shared a cache identity")
	}
	for _, token := range []string{"t-a0-s0-none-t0-b0-z0x1080", "t-a0-s0-none-t0-b0-z640x0", "t-a0-s0-none-t0-b0-z640x1082", "t-a0-s0-none-t0-b0-z641x360", "t-a0-s0-none-t0-b0-zx360", "r-a0-s0-none-t0-b0-z640x360", "a-a0-s0-text-t0-b0", "r-a0-s0-none-t1-b0", strings.Repeat("x", 8193)} {
		if _, err := ParseHLSRecipe(token, HLSRecipePolicy{MaxBitrate: 100000000, OffsetStepMilliseconds: 100}); err == nil {
			t.Fatalf("accepted invalid recipe %q", token)
		}
	}
}

func TestSourceResolutionUsesSubtitleOrdinalAndScannedSidecar(t *testing.T) {
	facts := MediaFacts{Duration: 100, Video: VideoFacts{Codec: "h264"}, Audio: []AudioFacts{{Index: 0, SourceIndex: 1, Codec: "aac"}}, Subtitles: []SubtitleFacts{{Index: 0, SourceIndex: 2, Text: true}, {Index: 1, SourceIndex: 5, Text: true}, {Index: 2, SourceIndex: -1, Text: true, External: true, ExternalIndex: 0}}}
	recipe, err := ResolveHLSSource(HLSRecipe{Mode: "transcode", Burn: "text", Subtitle: 5}, facts, []string{"/library/movie.en.srt"})
	if err != nil || recipe.Subtitle != 5 || recipe.SubtitleOrdinal != 1 {
		t.Fatalf("resolved = %#v, %v", recipe, err)
	}
	mapping, _ := ApplyBurnIn([]string{"-vf", "scale=w=640:h=-2"}, "/library/movie.mkv", recipe, HLSRecipePolicy{})
	if !strings.Contains(strings.Join(mapping, " "), ":si=1") {
		t.Fatalf("absolute index leaked into subtitle filter: %v", mapping)
	}
	external, err := ResolveHLSSource(HLSRecipe{Mode: "transcode", Burn: "external", Subtitle: 0, SubtitlePath: "/untrusted"}, facts, []string{"/library/movie.en.srt"})
	if err != nil || external.SubtitlePath != "/library/movie.en.srt" {
		t.Fatalf("external = %#v, %v", external, err)
	}
	for _, invalid := range []HLSRecipe{{Mode: "transcode", Audio: 1}, {Mode: "transcode", Burn: "text", Subtitle: 1}, {Mode: "transcode", Burn: "external", Subtitle: 1}, {Mode: "transcode", Burn: "image", Subtitle: 2}, {Mode: "remux", Burn: "text", Subtitle: 2}, {Mode: "transcode", Offset: math.NaN()}, {Mode: "transcode", Omitted: []Range{{0, 101}}}, {Mode: "transcode", Omitted: []Range{{5, 4}}}, {Mode: "transcode", Omitted: []Range{{0, 5}, {4, 10}}}, {Mode: "transcode", ToneMap: true}} {
		if _, err := ResolveHLSSource(invalid, facts, []string{"/library/movie.en.srt"}); err == nil {
			t.Fatalf("accepted source mismatch: %#v", invalid)
		}
	}
}

func TestSourceSpecificFramesColorAndDisplayGeometry(t *testing.T) { //nolint:cyclop // Each source plan checks both output geometry and color/frame arguments.
	options := transcodepolicy.Settings{Accelerator: "cuda", Encoder: "h264_nvenc", Codec: "h264", ToneMap: true, HardwareDecode: true}
	facts := MediaFacts{Video: VideoFacts{Codec: "h264", BitDepth: 8, Width: 720, Height: 576, SampleAspectRatio: "16:15", Rotation: 90, FieldOrder: "tt"}}
	actual := SourceTranscoding(options, facts, HLSRecipe{Mode: "transcode", Burn: "text"})
	if actual.ToneMap || actual.HardwareDecode || !actual.SoftwareFilters || !actual.Deinterlace || actual.Accelerator != "cuda" {
		t.Fatalf("SDR source options = %#v", actual)
	}
	w, h := DisplayDimensions(facts.Video)
	w, h = FitDimensions(w, h, 1920, 540)
	if w != 404 || h != 540 {
		t.Fatalf("display fit = %dx%d", w, h)
	}
	facts.Video.HDR = "hdr10"
	options.Codec = "hevc"
	hdr := SourceTranscoding(options, facts, HLSRecipe{Mode: "transcode"})
	if hdr.OutputHDR != "hdr10" || hdr.ToneMap {
		t.Fatalf("HDR preserve = %#v", hdr)
	}
	sdr := SourceTranscoding(options, facts, HLSRecipe{Mode: "transcode", ToneMap: true})
	if sdr.OutputHDR != "" || !sdr.ToneMap {
		t.Fatalf("HDR conversion = %#v", sdr)
	}
}

func TestMasterUsesActualInitializationInsteadOfGuesses(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "360p")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "init.mp4"), mp4fixture.Initialization(640, 360, "hevc", "opus", "hlg"), 0o600); err != nil {
		t.Fatal(err)
	}
	var manifest []byte
	writer := func(_ string, data []byte) error { manifest = append([]byte(nil), data...); return nil }
	if err := WriteMaster(filepath.Join(root, "index.m3u8"), "encoder", "avc1.64002a,mp4a.40.2", []PlaybackQuality{{Label: "360p", Width: 1920, Height: 1080, Bitrate: 1000000}}, true, writer); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`CODECS="hvc1.2.6.L120.B0,opus"`, `VIDEO-RANGE=HLG`, `RESOLUTION=640x360`} {
		if !strings.Contains(string(manifest), expected) {
			t.Fatalf("manifest lacks %q: %s", expected, manifest)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "init.mp4"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest = nil
	if err := WriteMaster(filepath.Join(root, "index.m3u8"), "encoder", "avc1", []PlaybackQuality{{Label: "360p", Width: 640, Height: 360, Bitrate: 1}}, true, writer); err == nil || manifest != nil {
		t.Fatal("invalid initialization was published")
	}
}

func TestClientProfilesKeepCombinationsAndOutputConstraints(t *testing.T) {
	facts := MediaFacts{Kind: "video", Container: "mp4", Video: VideoFacts{Codec: "vp9", Width: 1280, Height: 720, BitDepth: 8}, Audio: []AudioFacts{{Codec: "aac", Channels: 2}}}
	profile := JellyfinDeviceProfile{DirectPlayProfiles: []JellyfinMediaProfile{{Type: "Video", Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}, {Type: "Video", Container: "webm", VideoCodec: "vp9", AudioCodec: "opus"}}}
	client := JellyfinCapabilities(profile, facts)
	if directCombination(client, facts, facts.Audio[0]) {
		t.Fatal("flattened cross-combination was accepted")
	}
	facts.Video.Codec, facts.Video.Profile, facts.Video.Level = "h264", "High", "51"
	client = BrowserCapabilities()
	if playbackCompatibility(facts, client, facts.Audio[0], 0).video {
		t.Fatal("unsupported AVC level was accepted")
	}
	client.CodecProfiles = []JellyfinCodecProfile{{Type: "Video", Codec: "h264", Conditions: []JellyfinProfileCondition{{Condition: "LessThanEqual", Property: "VideoLevel", Value: "31", IsRequired: true}}}}
	plan := Decide(facts, client, ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, NetworkIntent{ForceTranscode: true}, DecisionPolicy{})
	if plan.Allowed || plan.Mode != "denied" {
		t.Fatalf("promised an output outside the decoder contract: %#v", plan)
	}
	for _, value := range []string{"NaN", "+Inf", "-1", "1000000000001", "garbage"} {
		invalid := []JellyfinCodecProfile{{Type: "Video", Conditions: []JellyfinProfileCondition{{Condition: "LessThanEqual", Property: "VideoLevel", Value: value}}}}
		if validCodecProfiles(invalid) {
			t.Fatalf("accepted invalid condition %q", value)
		}
	}
}

func TestAudioEvidenceRejectsInvalidQueriesBeforePlanning(t *testing.T) {
	for _, query := range []string{"audioCodecs=", "audioCodecs=aac&audioCodecs=opus", "audioCodecs=AAC", "audioCodecs=aac,aac", "audioCodecs=unknown", "audioCodecs=" + strings.Repeat("a", 65)} {
		if _, err := RequestedAudioCodecs(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playback?"+query, nil)); err == nil {
			t.Fatalf("accepted invalid audio evidence: %s", query)
		}
	}
	if actual, err := RequestedAudioCodecs(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playback?audioCodecs=aac,mp3", nil)); err != nil || !reflect.DeepEqual(actual, []string{"aac", "mp3"}) {
		t.Fatalf("audio evidence = %v, %v", actual, err)
	}
}
