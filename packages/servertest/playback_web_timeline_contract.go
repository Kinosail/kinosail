package servertest

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/playerweb"
)

// WebPlaybackFixture retains real app settings across repeated playback applications.
type WebPlaybackFixture func([]string, bool) func(*http.Request, playback.MediaFacts, *playerweb.PlayerData)

// WebPlaybackOffersButDoesNotAutoSkipUnreviewedCredits checks the real web playback operation.
func (fixture WebPlaybackFixture) WebPlaybackOffersButDoesNotAutoSkipUnreviewedCredits(t *testing.T) {
	t.Helper()
	apply := fixture([]string{"credits"}, true)
	marker := mediaprobe.Marker{Type: "credits", Label: "Credits", Start: 80, End: 100, Source: "visual"}
	data := playerweb.PlayerData{Item: library.Item{ID: "movie", Kind: "video"}, Duration: 100, Markers: []mediaprobe.Marker{marker}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/movie", nil)
	apply(request, playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 100, Video: playback.VideoFacts{Codec: "h264"}}, &data)
	if data.AutoSkip != "" || !reflect.DeepEqual(data.Markers, []mediaprobe.Marker{marker}) || data.Plan.MarkerMode == "server" {
		t.Fatalf("unreviewed web credits = auto %q, markers %#v, plan %#v", data.AutoSkip, data.Markers, data.Plan)
	}
}

// WebPlaybackStartsAfterATrustedOpeningWithoutTranscoding checks the real web playback operation.
func (fixture WebPlaybackFixture) WebPlaybackStartsAfterATrustedOpeningWithoutTranscoding(t *testing.T) {
	t.Helper()
	apply := fixture([]string{"intro"}, false)
	marker := mediaprobe.Marker{Type: "intro", Label: "Intro", Start: 0, End: 20, Source: "fingerprint"}
	data := playerweb.PlayerData{Item: library.Item{ID: "movie", Kind: "video"}, Duration: 120, Markers: []mediaprobe.Marker{marker}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/movie", nil)
	apply(request, playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 120, Video: playback.VideoFacts{Codec: "h264"}}, &data)
	if data.Start != 20 || data.AutoSkip != "intro" || data.Plan.MarkerMode != "" {
		t.Fatalf("direct opening start = %v, auto %q, plan %#v", data.Start, data.AutoSkip, data.Plan)
	}

	data.Start = 42
	apply(request, playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 120, Video: playback.VideoFacts{Codec: "h264"}}, &data)
	if data.Start != 42 {
		t.Fatalf("resume start = %v, want 42", data.Start)
	}
}

// AutomaticPlaybackKeepsSkipMarkersOnTheDirectTimeline checks the real web playback operation.
func (fixture WebPlaybackFixture) AutomaticPlaybackKeepsSkipMarkersOnTheDirectTimeline(t *testing.T) {
	t.Helper()
	apply := fixture([]string{"intro", "credits"}, true)
	credits := mediaprobe.Marker{Type: "credits", Label: "Credits", Start: 80, End: 100, Source: "visual"}
	data := playerweb.PlayerData{Item: library.Item{ID: "episode", Kind: "video"}, Duration: 100, Markers: []mediaprobe.Marker{{Type: "intro", Start: 10, End: 20, Source: "fingerprint"}, credits}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/episode", nil)
	facts := playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 100, Video: playback.VideoFacts{Codec: "h264", FrameRate: 24}, Audio: []playback.AudioFacts{{Codec: "aac"}}, RandomAccess: []float64{0, 10, 20, 100}}
	apply(request, facts, &data)
	want := []mediaprobe.Marker{{Type: "intro", Start: 10, End: 20, Source: "fingerprint"}, credits}
	if data.Plan.Mode != "direct" || data.Plan.MarkerMode != "" || data.Duration != 100 || data.AutoSkip != "intro" || !reflect.DeepEqual(data.Markers, want) || !reflect.DeepEqual(data.AdminMarkers, want) {
		t.Fatalf("direct marker skip = auto %q, duration %v, markers %#v, plan %#v", data.AutoSkip, data.Duration, data.Markers, data.Plan)
	}
}
