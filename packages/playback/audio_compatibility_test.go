package playback

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestAudioCompatibilityPreservesDirectAndConvertsFallback(t *testing.T) { //nolint:cyclop,gocognit // The table compares direct and fallback plans for the same source facts.
	for _, kind := range []string{"audio", "audiobook"} {
		for _, codec := range []string{"aac", "mp3", "flac", "alac", "opus", "vorbis", "wmav2", "ape", "wavpack", "dts", "pcm_s24le"} {
			facts := MediaFacts{Kind: kind, Container: "ogg", Duration: 30, Audio: []AudioFacts{{Index: 0, Codec: codec, Channels: 2}}, Video: VideoFacts{Codec: "mjpeg", Width: 600, Height: 600}}
			policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
			direct := Decide(facts, BrowserCapabilities(), policy, NetworkIntent{PreferDirect: true}, DecisionPolicy{})
			if !direct.Allowed || direct.Mode != "direct" {
				t.Fatalf("%s %s direct: %#v", kind, codec, direct)
			}
			fallback := Decide(facts, BrowserCapabilities(), policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{})
			if !fallback.Allowed || fallback.Mode != "audio-transcode" || fallback.VideoCodec != "" || fallback.AudioCodec != "aac" || fallback.Width != 0 || fallback.Height != 0 {
				t.Fatalf("%s %s fallback: %#v", kind, codec, fallback)
			}
			if _, err := ResolveHLSSource(RecipeFor(fallback), facts, nil); err != nil {
				t.Fatal(err)
			}
			policy.AllowTranscode = false
			denied := Decide(facts, BrowserCapabilities(), policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{})
			if denied.Allowed {
				t.Fatal("conversion allowed without permission")
			}
		}
	}
}

func TestAudioMasterDescribesOnlyObservedAAC(t *testing.T) { //nolint:cyclop // A manifest must match both observed streams and their published attributes.
	root := t.TempDir()
	directory := filepath.Join(root, "audio")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	// Strip the video track from the standard sample-description fixture.
	combined := mp4fixture.Initialization(640, 360, "h264", "aac", "")
	audio := mp4fixture.Box("moov", combined[8+binary.BigEndian.Uint32(combined[8:12]):])
	quality := PlaybackQuality{Label: "audio", Bitrate: 192000}
	for _, sample := range []struct {
		name  string
		data  []byte
		valid bool
	}{{"aac", audio, true}, {"video", combined, false}, {"empty", nil, false}, {"unknown", mp4fixture.Box("moov", mp4fixture.Track("Opus", make([]byte, 28))), false}} {
		t.Run(sample.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(directory, "init.mp4"), sample.data, 0o600); err != nil {
				t.Fatal(err)
			}
			var output []byte
			err := WriteMaster(filepath.Join(root, "index.m3u8"), "ffmpeg", "", []PlaybackQuality{quality}, false, func(_ string, data []byte) error { output = data; return nil })
			if sample.valid {
				if err != nil || !strings.Contains(string(output), "CODECS=\"mp4a.40.2\"\naudio/index.m3u8") || strings.Contains(string(output), "RESOLUTION") || strings.Contains(string(output), "FRAME-RATE") {
					t.Fatalf("master = %q, error = %v", output, err)
				}
			} else if err == nil || output != nil {
				t.Fatalf("invalid initialization published: %q, %v", output, err)
			}
		})
	}
}

func TestAudioRecipeRejectsVideoAndInvalidTrackFields(t *testing.T) {
	facts := MediaFacts{Kind: "audio", Duration: 30, Audio: []AudioFacts{{Index: 0, Codec: "flac"}}}
	for _, recipe := range []HLSRecipe{{Mode: "remux"}, {Mode: "transcode"}, {Mode: "audio-transcode", Audio: 1}, {Mode: "audio-transcode", Burn: "text"}, {Mode: "audio-transcode", Subtitle: 1}, {Mode: "audio-transcode", ToneMap: true}, {Mode: "audio-transcode", Width: 640, Height: 360}, {Mode: "audio-transcode", Offset: 31}, {Mode: "audio-transcode", Omitted: []Range{{Start: 1, End: 2}}}} {
		if _, err := ResolveHLSSource(recipe, facts, nil); err == nil {
			t.Fatalf("accepted %#v", recipe)
		}
	}
	facts.Audio = nil
	if _, err := ResolveHLSSource(HLSRecipe{Mode: "audio-transcode"}, facts, nil); err == nil {
		t.Fatal("accepted missing audio")
	}
}

func TestAudioCompatibilityHonorsClientDeliveryAndMissingTracks(t *testing.T) {
	facts := MediaFacts{Kind: "audio", Audio: []AudioFacts{{Codec: "flac", Channels: 2}}}
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	client := BrowserCapabilities()
	client.DeliveryProfiles = []JellyfinMediaProfile{{Type: "Audio", Container: "mp4", AudioCodec: "aac", Protocol: "hls"}}
	plan := Decide(facts, client, policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{})
	if !plan.Allowed || plan.Mode != "audio-transcode" {
		t.Fatalf("audio delivery: %#v", plan)
	}
	client.DeliveryProfiles[0].Type = "Video"
	if plan = Decide(facts, client, policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{}); plan.Allowed || plan.Mode != "denied" {
		t.Fatalf("unsupported delivery accepted: %#v", plan)
	}
	facts.Audio = nil
	if plan = Decide(facts, BrowserCapabilities(), policy, NetworkIntent{PreferCompatibility: true}, DecisionPolicy{}); plan.Allowed {
		t.Fatal("missing audio accepted for conversion")
	}
}

func TestAudioHLSArgumentsNeverRequireVideo(t *testing.T) { //nolint:cyclop // The argument table checks the complete audio-only encoder contract.
	input := HLSCodecInput{AudioOnly: true, Arguments: []string{"-i", "source.ogg"}, AudioRate: "192000", CopyInput: "0", Recipe: HLSRecipe{Mode: "audio-transcode", Audio: 1}}
	args, err := HLSCodecArguments(input)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map 0:a:1 -vn -sn -dn -c:a aac -ac 2 -b:a 192000") || strings.Contains(joined, "-c:v") || strings.Contains(joined, ":v:") {
		t.Fatal(joined)
	}
	for _, mode := range []string{"", "remux", "transcode", "unknown"} {
		input.Recipe.Mode = mode
		if args, err := HLSCodecArguments(input); err == nil || args != nil {
			t.Fatal("accepted invalid audio mode")
		}
	}
	for _, path := range []string{"audio/index.m3u8", "audio/init.mp4", "audio/segment-00000.m4s"} {
		if !HLSFile(path) {
			t.Fatal(path)
		}
	}
	for _, path := range []string{"Audio/index.m3u8", "audio/../init.mp4", "audio/file.mp3"} {
		if HLSFile(path) {
			t.Fatal(path)
		}
	}
}
