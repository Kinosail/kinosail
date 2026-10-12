package playback

import (
	"math"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/markers"
)

func TestAPIPlaybackMarkerResponseBounds(t *testing.T) {
	const duration = 304.96
	for _, test := range []struct {
		name       string
		start, end float64
		want       []markers.Marker
	}{
		{"ordinary", 10, 20, []markers.Marker{{Type: "credits", Label: "Credits", Start: 10, End: 20, Source: "visual"}}},
		{"exact endpoint", 300, duration, []markers.Marker{{Type: "credits", Label: "Credits", Start: 300, End: duration, Source: "visual"}}},
		{"observed sub-frame overrun", 300, 304.96799999999996, []markers.Marker{{Type: "credits", Label: "Credits", Start: 300, End: duration, Source: "visual"}}},
		{"larger overrun", 300, 400, []markers.Marker{{Type: "credits", Label: "Credits", Start: 300, End: duration, Source: "visual"}}},
		{"starts at endpoint", duration, 305, []markers.Marker{}},
		{"starts beyond endpoint", 305, 306, []markers.Marker{}},
		{"negative start", -1, 5, []markers.Marker{}},
		{"negative end", 0, -1, []markers.Marker{}},
		{"reversed", 20, 10, []markers.Marker{}},
		{"empty", 10, 10, []markers.Marker{}},
		{"NaN start", math.NaN(), 20, []markers.Marker{}},
		{"NaN end", 10, math.NaN(), []markers.Marker{}},
		{"infinite start", math.Inf(1), 20, []markers.Marker{}},
		{"infinite end", 10, math.Inf(1), []markers.Marker{}},
		{"negative infinite start", math.Inf(-1), 20, []markers.Marker{}},
		{"negative infinite end", 10, math.Inf(-1), []markers.Marker{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := []markers.Marker{{Type: "credits", Label: "Credits", Start: test.start, End: test.end, Source: "visual"}}
			response := &timelineResponse{}
			ApplyAPIPlaybackTimeline(response, PlaybackPlan{}, 42, duration, nil, input, nil, nil, func() string { t.Fatal("unexpected token creation"); return "" })
			if !reflect.DeepEqual(response.markers, test.want) || response.duration != duration || response.start != 42 || response.token != "" {
				t.Fatalf("response markers = %#v, want %#v; duration=%v start=%v token=%q", response.markers, test.want, response.duration, response.start, response.token)
			}
			if math.Float64bits(input[0].Start) != math.Float64bits(test.start) || math.Float64bits(input[0].End) != math.Float64bits(test.end) {
				t.Fatal("projection changed source marker metadata")
			}
		})
	}
}

func TestAPIPlaybackMarkerResponseInvalidDurations(t *testing.T) {
	for _, duration := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		response := &timelineResponse{}
		ApplyAPIPlaybackTimeline(response, PlaybackPlan{}, 0, duration, nil, []markers.Marker{{Type: "credits", Start: 1, End: 2}}, nil, nil, func() string { return "" })
		if len(response.markers) != 0 {
			t.Fatalf("invalid duration %v emitted markers %#v", duration, response.markers)
		}
	}
}

func TestAPIPlaybackMarkerResponseShortenedTimeline(t *testing.T) {
	response := &timelineResponse{}
	plan := PlaybackPlan{MarkerMode: "server", Timeline: Timeline{SourceDuration: 304.96, Duration: 294.96, Omitted: []Range{{0, 10}}}}
	values := []markers.Marker{{Type: "intro", End: 10, Source: "manual"}, {Type: "credits", Start: 300, End: 304.96799999999996, Source: "visual"}, {Type: "recap", Start: -1, End: 30, Source: "visual"}, {Type: "outro", Start: 300, End: math.Inf(1), Source: "visual"}}
	ApplyAPIPlaybackTimeline(response, plan, 20, 304.96, nil, values, []string{"intro"}, []string{"intro"}, func() string { return "recipe" })
	want := []markers.Marker{{Type: "credits", Start: 290, End: 294.96, Source: "visual"}}
	if !reflect.DeepEqual(response.markers, want) || response.duration != 294.96 || response.start != 10 || response.token != "recipe" || len(response.autoSkip) != 0 {
		t.Fatalf("shortened response = %#v", response)
	}
}
