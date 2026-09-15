package servertest

import (
	"math"
	"testing"

	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/playback"
)

// AutomaticSkipPlanner binds a real app's automatic-skip policy wrapper.
type AutomaticSkipPlanner func(playback.MediaFacts, playback.ClientCapabilities, playback.ViewerPolicy, playback.NetworkIntent, []mediaprobe.Marker, []string) playback.PlaybackPlan

// AssertAutomaticStartOffsetOnlyUsesATrustedIntroAtTheSourceStart checks the real app's automatic-skip boundary.
func AssertAutomaticStartOffsetOnlyUsesATrustedIntroAtTheSourceStart(t *testing.T, start func(float64, float64, []mediaprobe.Marker, []string) float64) {
	t.Helper()
	trusted := mediaprobe.Marker{Type: "intro", Start: 0, End: 20, Source: "fingerprint"}
	tests := []struct {
		name     string
		current  float64
		duration float64
		markers  []mediaprobe.Marker
		enabled  []string
		want     float64
	}{
		{"trusted opening", 0, 120, []mediaprobe.Marker{trusted}, []string{"intro"}, 20},
		{"resume", 42, 120, []mediaprobe.Marker{trusted}, []string{"intro"}, 42},
		{"disabled", 0, 120, []mediaprobe.Marker{trusted}, nil, 0},
		{"later intro", 0, 120, []mediaprobe.Marker{{Type: "intro", Start: 1, End: 20, Source: "fingerprint"}}, []string{"intro"}, 0},
		{"unreviewed", 0, 120, []mediaprobe.Marker{{Type: "intro", Start: 0, End: 20, Source: "recurrence"}}, []string{"intro"}, 0},
		{"unsafe sibling", 0, 120, []mediaprobe.Marker{trusted, {Type: "intro", Start: 0, End: 10, Source: "recurrence"}}, []string{"intro"}, 0},
		{"conflicting trusted markers", 0, 120, []mediaprobe.Marker{trusted, {Type: "intro", Start: 0, End: 10, Source: "manual"}}, []string{"intro"}, 0},
		{"ending too late", 0, 30, []mediaprobe.Marker{trusted}, []string{"intro"}, 0},
		{"invalid marker", 0, 120, []mediaprobe.Marker{{Type: "intro", Start: math.NaN(), End: 20, Source: "fingerprint"}}, []string{"intro"}, 0},
		{"invalid progress", math.Inf(1), 120, []mediaprobe.Marker{trusted}, []string{"intro"}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := start(test.current, test.duration, test.markers, test.enabled); got != test.want {
				t.Fatalf("start offset = %v, want %v", got, test.want)
			}
		})
	}
}

// AssertAutomaticSkipFallbackPreservesNoTranscodePolicy checks the real app's automatic-skip boundary.
func AssertAutomaticSkipFallbackPreservesNoTranscodePolicy(t *testing.T, decide AutomaticSkipPlanner) {
	t.Helper()
	facts := playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 60, Video: playback.VideoFacts{Codec: "h264"}, Audio: []playback.AudioFacts{{Codec: "aac"}}}
	client := playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}
	plan := decide(facts, client, playback.ViewerPolicy{AllowPlayback: true}, playback.NetworkIntent{}, []mediaprobe.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}}, []string{"intro"})
	if plan.Mode != "direct" || plan.MarkerMode != "unavailable" {
		t.Fatalf("no-transcode plan = %#v", plan)
	}
}

// AssertAutomaticSkipCopiesVideoOnlyAtRandomAccessBoundaries checks the real app's automatic-skip boundary.
func AssertAutomaticSkipCopiesVideoOnlyAtRandomAccessBoundaries(t *testing.T, plan AutomaticSkipPlanner) {
	t.Helper()
	facts := playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 60, Video: playback.VideoFacts{Codec: "h264", FrameRate: 24}, Audio: []playback.AudioFacts{{Codec: "aac"}}, RandomAccess: []float64{0, 10, 20, 60}}
	client := playback.ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}
	marker := []mediaprobe.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}}
	policy := playback.ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	if plan := plan(facts, client, policy, playback.NetworkIntent{}, marker, []string{"intro"}); plan.Mode != "audio-transcode" || plan.Reason != "automatic-marker-skip-remux" || plan.Timeline.Duration != 50 {
		t.Fatalf("aligned automatic skip plan = %#v", plan)
	}
	facts.RandomAccess[1] = 8
	if plan := plan(facts, client, policy, playback.NetworkIntent{}, marker, []string{"intro"}); plan.Mode != "transcode" || plan.Reason != "automatic-marker-skip" {
		t.Fatalf("unaligned automatic skip plan = %#v", plan)
	}
}
