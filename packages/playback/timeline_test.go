package playback

import (
	"math"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestAutomaticSkipTimelineNormalizesTrustedRanges(t *testing.T) {
	values := []markers.Marker{
		{Type: "intro", Start: 10, End: 20, Source: "manual"},
		{Type: "commercial", Start: 15, End: 25, Source: "chapter"},
		{Type: "credits", Start: 90, End: 110, Source: "fingerprint"},
		{Type: "recap", Start: 40, End: 50, Source: "manual"},
		{Type: "intro", Start: -1, End: 2, Source: "manual"},
		{Type: "intro", Start: 30, End: 30, Source: "manual"},
		{Type: "intro", Start: 101, End: 110, Source: "manual"},
		{Type: "intro", Start: 30, End: 40, Source: "visual"},
	}
	got := TimelineForAutomaticSkip(100, values, []string{"intro", "commercial", "credits"})
	want := Timeline{SourceDuration: 100, Duration: 75, Omitted: []Range{{10, 25}, {90, 100}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("timeline = %#v, want %#v", got, want)
	}
	whole := TimelineForAutomaticSkip(60, []markers.Marker{{Type: "intro", End: 60, Source: "manual"}}, []string{"intro"})
	if whole.Duration != 60 || len(whole.Omitted) != 0 {
		t.Fatalf("whole timeline = %#v", whole)
	}
}

func TestAutomaticSkipTypesAndStartRejectUntrustedState(t *testing.T) {
	trusted := markers.Marker{Type: "intro", End: 20, Source: "fingerprint"}
	if got := AutomaticSkipTypes([]markers.Marker{trusted}, []string{"intro", "credits"}); !reflect.DeepEqual(got, []string{"intro"}) {
		t.Fatalf("skip types = %#v", got)
	}
	if got := AutomaticSkipTypes([]markers.Marker{trusted, {Type: "intro", End: 10, Source: "visual"}}, []string{"intro"}); len(got) != 0 {
		t.Fatalf("unsafe skip types = %#v", got)
	}
	for name, test := range map[string]struct {
		current, duration float64
		values            []markers.Marker
		enabled           []string
		want              float64
	}{
		"trusted":        {0, 120, []markers.Marker{{Type: "recap"}, trusted}, []string{"intro"}, 20},
		"resume":         {42, 120, []markers.Marker{trusted}, []string{"intro"}, 42},
		"invalid input":  {math.Inf(1), 120, []markers.Marker{trusted}, []string{"intro"}, 0},
		"invalid length": {0, math.NaN(), []markers.Marker{trusted}, []string{"intro"}, 0},
		"invalid marker": {0, 120, []markers.Marker{{Type: "intro", Start: math.NaN(), End: 20, Source: "manual"}}, []string{"intro"}, 0},
		"ending late":    {0, 30, []markers.Marker{trusted}, []string{"intro"}, 0},
		"conflict":       {0, 120, []markers.Marker{trusted, {Type: "intro", End: 10, Source: "manual"}}, []string{"intro"}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if got := AutomaticStart(test.current, test.duration, test.values, test.enabled); got != test.want {
				t.Fatalf("start = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTimelineMapsMarkersAndChapters(t *testing.T) {
	timeline := Timeline{SourceDuration: 100, Duration: 80, Omitted: []Range{{10, 20}, {40, 50}}}
	values := []markers.Marker{
		{Type: "intro", Start: 10, End: 20, Source: "manual"},
		{Type: "credits", Start: 80, End: 100, Source: "visual"},
		{Type: "recap", Start: 12, End: 18, Source: "visual"},
	}
	got := RemainingMarkers(values, []string{"intro"}, timeline)
	want := []markers.Marker{{Type: "credits", Start: 60, End: 80, Source: "visual"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("markers = %#v, want %#v", got, want)
	}
	chapters := TimelineChapters(timeline, []metadata.Chapter{{Index: 4, Start: 0, End: 10}, {Index: 5, Start: 12, End: 18}, {Index: 6, Start: 20, End: 40}})
	wantChapters := []metadata.Chapter{{Index: 0, Start: 0, End: 10}, {Index: 1, Start: 10, End: 30}}
	if !reflect.DeepEqual(chapters, wantChapters) {
		t.Fatalf("chapters = %#v, want %#v", chapters, wantChapters)
	}
}

func TestDecideWithAutomaticSkipPreservesPolicyAndChoosesDelivery(t *testing.T) { //nolint:cyclop // One matrix protects every delivery branch.
	facts := MediaFacts{Kind: "video", Container: "mp4", Duration: 60, Video: VideoFacts{Codec: "h264", FrameRate: 24}, Audio: []AudioFacts{{Codec: "aac"}}, RandomAccess: []float64{0, 10, 20, 60}}
	client := ClientCapabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, SupportsRemux: true}
	policy := ViewerPolicy{AllowPlayback: true, AllowTranscode: true}
	values := []markers.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}}
	enabled := []string{"intro"}
	if plan := DecideWithAutomaticSkip(facts, client, policy, NetworkIntent{PreferDirect: true}, values, enabled, DecisionPolicy{}); plan.MarkerMode != "" {
		t.Fatalf("direct plan = %#v", plan)
	}
	if plan := DecideWithAutomaticSkip(facts, client, policy, NetworkIntent{}, nil, enabled, DecisionPolicy{}); plan.MarkerMode != "" {
		t.Fatalf("plain plan = %#v", plan)
	}
	if plan := DecideWithAutomaticSkip(facts, client, ViewerPolicy{AllowPlayback: true}, NetworkIntent{}, values, enabled, DecisionPolicy{}); plan.MarkerMode != "unavailable" {
		t.Fatalf("no-transcode plan = %#v", plan)
	}
	if plan := DecideWithAutomaticSkip(facts, client, policy, NetworkIntent{}, values, enabled, DecisionPolicy{}); plan.Mode != "audio-transcode" || plan.MarkerMode != "server" || plan.Reason != "automatic-marker-skip-remux" {
		t.Fatalf("aligned plan = %#v", plan)
	}
	facts.Audio = nil
	if plan := DecideWithAutomaticSkip(facts, client, policy, NetworkIntent{}, values, enabled, DecisionPolicy{}); plan.Mode != "remux" {
		t.Fatalf("video-only plan = %#v", plan)
	}
	facts.RandomAccess[1] = 8
	if plan := DecideWithAutomaticSkip(facts, client, policy, NetworkIntent{}, values, enabled, DecisionPolicy{}); plan.Mode != "transcode" || plan.MarkerMode != "server" || plan.Reason != "automatic-marker-skip" {
		t.Fatalf("transcode plan = %#v", plan)
	}
}

func TestTimelineAlignmentAndJellyfinProjectionCoverEdges(t *testing.T) { //nolint:cyclop // One matrix protects all timeline alignment edges.
	timeline := Timeline{SourceDuration: 60, Duration: 50, Omitted: []Range{{10, 20}}}
	if _, ok := randomAccessTimeline(timeline, nil, 0); ok {
		t.Fatal("empty random access points aligned")
	}
	if _, ok := randomAccessTimeline(timeline, []float64{0, 8, 20, 60}, 24); ok {
		t.Fatal("distant point aligned")
	}
	invalid := Timeline{SourceDuration: 60, Duration: 60, Omitted: []Range{{20, 10}}}
	if _, ok := randomAccessTimeline(invalid, []float64{0, 10, 20, 60}, 0); ok {
		t.Fatal("reversed range aligned")
	}
	full := Timeline{SourceDuration: 60, Duration: 0, Omitted: []Range{{0, 60}}}
	if _, ok := randomAccessTimeline(full, []float64{0, 60}, 0); ok {
		t.Fatal("empty presentation aligned")
	}
	if point, ok := randomAccessPoint(10, 60, []float64{40, 10.001}, 0.002); !ok || point != 10.001 {
		t.Fatalf("nearest point = %v, %v", point, ok)
	}
	if point, ok := randomAccessPoint(10, 60, []float64{9}, 0.002); ok || point != 9 {
		t.Fatalf("distant point = %v, %v", point, ok)
	}
	values := []markers.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}, {Type: "credits", Start: 50, End: 60, Source: "visual"}}
	if got := JellyfinMarkers(60, values, []string{"intro"}, false); !reflect.DeepEqual(got, values) {
		t.Fatalf("source markers = %#v", got)
	}
	if got := JellyfinMarkers(60, values, []string{"intro"}, true); len(got) != 1 || got[0].Start != 40 {
		t.Fatalf("presentation markers = %#v", got)
	}
}

type timelineResponse struct {
	duration, start float64
	chapters        []metadata.Chapter
	markers         []markers.Marker
	autoSkip        []string
	token           string
}

func (response *timelineResponse) SetPlaybackTimeline(duration, start float64, chapters []metadata.Chapter, values []markers.Marker, autoSkip []string, token string) {
	response.duration, response.start, response.chapters, response.markers, response.autoSkip, response.token = duration, start, chapters, values, autoSkip, token
}

func TestApplyAPIPlaybackTimelineProjectsServerAndClientModes(t *testing.T) { //nolint:cyclop // The score of 15 remains below the repository ceiling of 22 for the mode matrix.
	t.Parallel()
	values := []markers.Marker{{Type: "intro", Start: 10, End: 20, Source: "manual"}}
	chapters := []metadata.Chapter{{Start: 0, End: 30}}
	response, tokens := &timelineResponse{}, 0
	plan := PlaybackPlan{MarkerMode: "server", Timeline: Timeline{SourceDuration: 60, Duration: 50, Omitted: []Range{{10, 20}}}}
	ApplyAPIPlaybackTimeline(response, plan, 25, 60, chapters, values, []string{"intro"}, []string{"intro"}, func() string { tokens++; return "recipe" })
	if response.duration != 50 || response.start != 15 || len(response.chapters) != 1 || len(response.markers) != 0 || len(response.autoSkip) != 0 || response.token != "recipe" || tokens != 1 {
		t.Fatalf("server response = %#v tokens=%d", response, tokens)
	}
	values = []markers.Marker{{Type: "intro", Start: 0, End: 5, Source: "manual"}}
	ApplyAPIPlaybackTimeline(response, PlaybackPlan{}, 0, 60, chapters, values, []string{"intro"}, []string{"intro"}, func() string { tokens++; return "unused" })
	if response.duration != 60 || response.start != 5 || !reflect.DeepEqual(response.chapters, chapters) || !reflect.DeepEqual(response.markers, values) || !reflect.DeepEqual(response.autoSkip, []string{"intro"}) || response.token != "" || tokens != 1 {
		t.Fatalf("client response = %#v tokens=%d", response, tokens)
	}
}
