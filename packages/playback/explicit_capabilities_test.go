package playback

import "testing"

func TestExplicitHDRSupportRequiresMatchingVideoRangeCondition(t *testing.T) {
	t.Parallel()
	video := VideoFacts{Codec: "hevc", HDR: "hdr10"}
	for _, test := range []struct {
		kind, codec, property, condition, value string
		want                                    bool
	}{
		{"Audio", "hevc", "VideoRangeType", "Equals", "HDR10", false},
		{"Video", "h264", "VideoRangeType", "Equals", "HDR10", false},
		{"Video", "hevc", "VideoCodec", "Equals", "HDR10", false},
		{"Video", "hevc", "VideoRangeType", "NotEquals", "SDR", false},
		{"Video", "hevc", "VideoRangeType", "Equals", "SDR", false},
		{"Video", "hevc", "VideoRangeType", "Equals", "HDR10", true},
		{"video", "", "VideoRangeType", "EqualsAny", "SDR|HDR10", true},
	} {
		profile := JellyfinCodecProfile{Type: test.kind, Codec: test.codec, Conditions: []JellyfinProfileCondition{{Property: test.property, Condition: test.condition, Value: test.value}}}
		if got := explicitHDRSupport([]JellyfinCodecProfile{profile}, video); got != test.want {
			t.Fatalf("%#v support=%v", profile, got)
		}
	}
	if explicitHDRSupport(nil, video) {
		t.Fatal("absent HDR capability accepted")
	}
}

func TestRemuxAudioChecksOnlyTheSelectedTrack(t *testing.T) {
	t.Parallel()
	for _, codec := range []string{"aac", "mp3", "opus", "ac3", "eac3", "flac", "dts", "pcm_s16le"} {
		tracks := []AudioFacts{{Index: 0, Codec: "dts"}, {Index: 1, Codec: codec}}
		want := codec != "dts" && codec != "pcm_s16le"
		if got := validRemuxAudio(HLSRecipe{Mode: "remux", Audio: 1}, tracks); got != want {
			t.Fatalf("remux %s=%v", codec, got)
		}
		if !validRemuxAudio(HLSRecipe{Mode: "transcode", Audio: 1}, tracks) {
			t.Fatalf("transcode rejected %s", codec)
		}
	}
}
