package server

import (
	"reflect"
	"testing"

	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAutomaticStartOffsetOnlyUsesATrustedIntroAtTheSourceStart(t *testing.T) {
	servertest.AssertAutomaticStartOffsetOnlyUsesATrustedIntroAtTheSourceStart(t, automaticStartOffset)
}

func TestAutomaticSkipNeverRemovesUnreviewedDetectedCredits(t *testing.T) {
	markers := []playbackMarker{{Type: "credits", Start: 80, End: 100}}
	for _, source := range []string{"", "visual", "model", "recurrence"} {
		markers[0].Source = source
		if timeline := automaticSkipTimeline(100, markers, []string{"credits"}); len(timeline.Omitted) != 0 || timeline.Duration != 100 {
			t.Fatalf("%q credits timeline = %#v", source, timeline)
		}
		if enabled := automaticSkipSelection(markers, []string{"credits"}); len(enabled) != 0 {
			t.Fatalf("%q credits auto skip = %#v", source, enabled)
		}
	}
	markers[0].Source = "manual"
	if timeline := automaticSkipTimeline(100, markers, []string{"credits"}); !reflect.DeepEqual(timeline.Omitted, []PlaybackRange{{Start: 80, End: 100}}) {
		t.Fatalf("confirmed credits timeline = %#v", timeline)
	}
}

func TestWebPlaybackOffersButDoesNotAutoSkipUnreviewedCredits(t *testing.T) {
	webPlaybackTimeline.WebPlaybackOffersButDoesNotAutoSkipUnreviewedCredits(t)
}

func TestWebPlaybackStartsAfterATrustedOpeningWithoutTranscoding(t *testing.T) {
	webPlaybackTimeline.WebPlaybackStartsAfterATrustedOpeningWithoutTranscoding(t)
}

func TestAutomaticPlaybackKeepsSkipMarkersOnTheDirectTimeline(t *testing.T) {
	webPlaybackTimeline.AutomaticPlaybackKeepsSkipMarkersOnTheDirectTimeline(t)
}

func TestJellyfinOnlyExposesAutoEligibleMarkers(t *testing.T) {
	segments := markerlogic.JellyfinSegments("episode", []playbackMarker{
		{Type: "intro", Start: 10, End: 20, Source: "fingerprint"},
		{Type: "credits", Start: 80, End: 100, Source: "visual"},
	})
	if len(segments) != 1 || segments[0].Type != "Intro" || segments[0].ItemID != "episode" {
		t.Fatalf("Jellyfin segments = %#v", segments)
	}
}

func TestAutomaticSkipFallbackPreservesNoTranscodePolicy(t *testing.T) {
	servertest.AssertAutomaticSkipFallbackPreservesNoTranscodePolicy(t, playbackWithAutomaticSkip)
}

func TestAutomaticSkipCopiesVideoOnlyAtRandomAccessBoundaries(t *testing.T) {
	servertest.AssertAutomaticSkipCopiesVideoOnlyAtRandomAccessBoundaries(t, playbackWithAutomaticSkip)
}

func TestAutomaticSkipRecipeRoundTripsManyRanges(t *testing.T) {
	ranges := make([]PlaybackRange, 0, 12)
	for index := 0; index < 12; index++ {
		ranges = append(ranges, PlaybackRange{Start: float64(index * 10), End: float64(index*10 + 5)})
	}
	want := hlsRecipe{mode: "transcode", codec: "av1", audio: 1, subtitle: 3, omitted: ranges, offset: 30}
	shared, err := playback.ParseHLSRecipe(want.token(), hlsPolicy())
	got := localHLSRecipe(shared)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("recipe round trip = %#v, %v; want %#v", got, err, want)
	}
}
