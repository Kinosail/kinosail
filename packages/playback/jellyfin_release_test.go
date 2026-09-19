package playback

import "testing"

func TestJellyfinRotationConstraintSelectsCompatibleOutput(t *testing.T) {
	for _, rotation := range []int{-90, 0, 90, 180, 270} {
		facts := MediaFacts{Kind: "video", Container: "mp4", Video: VideoFacts{Codec: "h264", Width: 1280, Height: 720, Rotation: rotation}}
		profile := JellyfinDeviceProfile{DirectPlayProfiles: []JellyfinMediaProfile{{Type: "Video", Container: "mp4", VideoCodec: "h264"}}, TranscodingProfiles: []JellyfinMediaProfile{{Type: "Video", Container: "mp4", VideoCodec: "h264", Protocol: "hls"}}, CodecProfiles: []JellyfinCodecProfile{{Type: "Video", Conditions: []JellyfinProfileCondition{{Property: "VideoRotation", Condition: "Equals", Value: "0", IsRequired: true}}}}}
		plan := Decide(facts, JellyfinCapabilities(profile, facts), ViewerPolicy{AllowPlayback: true, AllowTranscode: true}, NetworkIntent{}, DecisionPolicy{})
		want := "transcode"
		if rotation == 0 {
			want = "direct"
		}
		if !plan.Allowed || plan.Mode != want {
			t.Fatalf("rotation %d: %#v", rotation, plan)
		}
		if rotation != 0 {
			denied := Decide(facts, JellyfinCapabilities(profile, facts), ViewerPolicy{AllowPlayback: true}, NetworkIntent{}, DecisionPolicy{})
			if denied.Allowed {
				t.Fatalf("rotation %d bypassed conversion permission", rotation)
			}
		}
	}
}

func TestJellyfinDimensionsUseWirePropertyNames(t *testing.T) {
	facts := MediaFacts{Video: VideoFacts{Width: 1920, Height: 1080}}
	for _, property := range []string{"Width", "Height"} {
		conditions := []JellyfinProfileCondition{{Property: property, Condition: "LessThanEqual", Value: "720", IsRequired: false}}
		if matchesProfileConditions(conditions, facts, AudioFacts{}) {
			t.Fatalf("ignored %s constraint", property)
		}
	}
}

func TestJellyfinSignedRotationConditions(t *testing.T) {
	for _, value := range []string{"-360", "-90", "0", "90", "360", "-90|0|90"} {
		condition := JellyfinProfileCondition{Property: "VideoRotation", Condition: "EqualsAny", Value: value}
		if !validProfileCondition(condition) {
			t.Fatalf("rejected %q", value)
		}
	}
	for _, value := range []string{"", "NaN", "-361", "361", "45", "90.0", "+90", "090", "0|", "unknown"} {
		if validProfileCondition(JellyfinProfileCondition{Property: "VideoRotation", Condition: "EqualsAny", Value: value}) {
			t.Fatalf("accepted %q", value)
		}
	}
	condition := JellyfinProfileCondition{Property: "VideoRotation", Condition: "GreaterThanEqual", Value: "-90"}
	if !validProfileCondition(condition) || !matchesCondition(condition, "0") {
		t.Fatal("signed rotation comparison failed")
	}
}
