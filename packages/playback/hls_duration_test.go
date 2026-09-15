package playback

import "testing"

func TestHLSPlaybackDurationUsesPlayerSkipAndSeekTimeline(t *testing.T) {
	for _, test := range []struct {
		name         string
		recipe       HLSRecipe
		source, want float64
	}{
		{"unchanged", HLSRecipe{}, 30, 30},
		{"skip intro", HLSRecipe{Omitted: []Range{{Start: 10, End: 20}}}, 30, 20},
		{"seek across intro", HLSRecipe{Omitted: []Range{{Start: 10, End: 20}}, Offset: 15}, 30, 5},
		{"seek before intro", HLSRecipe{Omitted: []Range{{Start: 10, End: 20}}, Offset: 5}, 30, 15},
		{"unknown source", HLSRecipe{}, 0, 0},
		{"negative source clamps", HLSRecipe{}, -1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := HLSPlaybackDuration(test.recipe, test.source); got != test.want {
				t.Fatalf("duration = %v; want %v", got, test.want)
			}
		})
	}
}
