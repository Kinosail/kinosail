package playback

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/markers"
)

func surroundClient() ClientCapabilities {
	return ClientCapabilities{Containers: []string{"mp4", "mov"}, VideoCodecs: []string{"h264", "hevc"}, AudioCodecs: []string{"aac", "ac3", "eac3"}, TranscodeVideoCodecs: []string{"h264"}, SupportsRemux: true}
}

func surroundFacts(codec string, channels int) MediaFacts {
	return MediaFacts{
		Kind: "video", Container: "matroska", Duration: 5400,
		Video: VideoFacts{Codec: "h264", Width: 1920, Height: 1080, FrameRate: 23.976},
		Audio: []AudioFacts{{Index: 0, Codec: codec, Channels: channels, Default: true}},
	}
}

func TestSurroundSourcesKeepSurroundAsEAC3WhenTheClientPlaysIt(t *testing.T) {
	t.Parallel()
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	for _, codec := range []string{"dts", "truehd"} {
		for _, channels := range []int{6, 8} {
			plan := Decide(surroundFacts(codec, channels), surroundClient(), policy, NetworkIntent{}, DecisionPolicy{})
			if !plan.Allowed || plan.Mode != "audio-transcode" || plan.AudioCodec != "eac3" {
				t.Fatalf("%s %dch plan = %#v", codec, channels, plan)
			}
			recipe := RecipeFor(plan)
			if !recipe.Surround || !strings.HasPrefix(recipe.Token(), "s-") {
				t.Fatalf("%s %dch recipe = %#v token %q", codec, channels, recipe, recipe.Token())
			}
			parsed, err := ParseHLSRecipe(recipe.Token(), HLSRecipePolicy{MaxBitrate: 1 << 40, OffsetStepMilliseconds: 1})
			if err != nil || !parsed.Surround || parsed.Mode != "audio-transcode" {
				t.Fatalf("%s %dch round trip = %#v, %v", codec, channels, parsed, err)
			}
		}
	}
}

func TestStereoOrLimitedClientsKeepAACStereo(t *testing.T) {
	t.Parallel()
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	noEAC3 := surroundClient()
	noEAC3.AudioCodecs = []string{"aac"}
	twoChannels := surroundClient()
	twoChannels.MaxAudioChannels = 2
	music := surroundFacts("dts", 6)
	music.Kind, music.Video = "audio", VideoFacts{}
	for name, test := range map[string]struct {
		facts  MediaFacts
		client ClientCapabilities
	}{
		"stereo source":   {surroundFacts("dts", 2), surroundClient()},
		"no E-AC-3":       {surroundFacts("dts", 6), noEAC3},
		"two-channel cap": {surroundFacts("truehd", 8), twoChannels},
		"music":           {music, surroundClient()},
	} {
		plan := Decide(test.facts, test.client, policy, NetworkIntent{}, DecisionPolicy{})
		if !plan.Allowed || plan.Mode != "audio-transcode" || plan.AudioCodec != "aac" || RecipeFor(plan).Surround {
			t.Errorf("%s: plan = %#v", name, plan)
		}
	}
}

func TestSurroundRecipeEncodesEAC3FiveOneUnlessAnAudioEffectIsOn(t *testing.T) {
	t.Parallel()
	input := HLSCodecInput{Arguments: []string{"-i", "movie.mkv"}, AudioRate: "192000", CopyInput: "0", Recipe: HLSRecipe{Mode: "audio-transcode", Surround: true}}
	args, err := HLSCodecArguments(input)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-c:v copy") || !strings.Contains(joined, "-c:a eac3 -ac 6 -b:a 640000") || strings.Contains(joined, "-c:a aac") {
		t.Fatalf("surround arguments = %s", joined)
	}
	input.Recipe.DialogueBoost = true
	args, err = HLSCodecArguments(input)
	if err != nil || !strings.Contains(strings.Join(args, " "), "-c:a aac -ac 2") {
		t.Fatalf("dialogue boost arguments = %v, %v", args, err)
	}
}

func TestSurroundIsOnlyValidForAudioConversion(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 1 << 40, OffsetStepMilliseconds: 1}
	for _, token := range []string{"s-a0-s0-none-t1-b0", "s-a0-s0-text-t0-b0", "s-a0-s0-none-t0-b0-z640x360"} {
		if _, err := ParseHLSRecipe(token, policy); err == nil {
			t.Errorf("accepted %q", token)
		}
	}
	if recipe, err := ParseHLSRecipe("s-a1-s0-none-t0-b0", policy); err != nil || !recipe.Surround || recipe.Audio != 1 {
		t.Fatalf("valid surround token = %#v, %v", recipe, err)
	}
}

func TestSkippedIntroKeepsSurroundForEAC3Clients(t *testing.T) {
	t.Parallel()
	facts := surroundFacts("ac3", 6)
	facts.Duration, facts.RandomAccess = 60, []float64{0, 10, 20, 60}
	values := []markers.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}}
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	plan := DecideWithAutomaticSkip(facts, surroundClient(), policy, NetworkIntent{}, values, []string{"intro"}, DecisionPolicy{})
	if plan.MarkerMode != "server" || plan.Mode != "audio-transcode" || plan.AudioCodec != "eac3" || !RecipeFor(plan).Surround {
		t.Fatalf("skip plan = %#v", plan)
	}
	stereo := surroundClient()
	stereo.MaxAudioChannels = 2
	if plan := DecideWithAutomaticSkip(facts, stereo, policy, NetworkIntent{}, values, []string{"intro"}, DecisionPolicy{}); plan.AudioCodec != "aac" {
		t.Fatalf("stereo skip plan = %#v", plan)
	}
}
