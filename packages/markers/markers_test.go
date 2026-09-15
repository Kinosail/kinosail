package markers

import (
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestDetectPlaybackMarkersClassifiesNamedChapters(t *testing.T) {
	t.Parallel()
	chapters := []metadata.Chapter{
		{Index: 0, Start: 0, End: 45, Title: "Previously on Kinosail"},
		{Index: 1, Start: 45, End: 105, Title: "Opening Credits"},
		{Index: 2, Start: 105, End: 900, Title: "Episode"},
		{Index: 3, Start: 900, End: 930, Title: "Ad Break"},
		{Index: 4, Start: 1800, End: 1840, Title: "Outro"},
		{Index: 5, Start: 1840, End: 1920, Title: "End Credits"},
	}
	want := []Marker{
		{Type: "recap", Label: "Recap", Start: 0, End: 45, Source: "chapter"},
		{Type: "intro", Label: "Intro", Start: 45, End: 105, Source: "chapter"},
		{Type: "commercial", Label: "Commercial", Start: 900, End: 930, Source: "chapter"},
		{Type: "outro", Label: "Outro", Start: 1800, End: 1840, Source: "chapter"},
		{Type: "credits", Label: "Credits", Start: 1840, End: 1920, Source: "chapter"},
	}
	if got := DetectPlaybackMarkers(chapters); !reflect.DeepEqual(got, want) {
		t.Fatalf("markers = %#v, want %#v", got, want)
	}
}

func TestDetectPlaybackMarkersIgnoresOrdinaryChapters(t *testing.T) {
	t.Parallel()
	chapters := []metadata.Chapter{{Title: "Chapter 1"}, {Title: "Prologue"}, {Title: "Director commentary"}}
	if got := DetectPlaybackMarkers(chapters); len(got) != 0 {
		t.Fatalf("markers = %#v", got)
	}
}

func TestNormalizeAutoSkip(t *testing.T) {
	t.Parallel()
	want := []string{"intro", "recap", "credits"}
	got, err := NormalizeAutoSkip([]string{" credits ", "INTRO", "intro", "recap", ""})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("auto skip = %#v, %v", got, err)
	}
	if _, err := NormalizeAutoSkip([]string{"intro", "everything"}); err == nil {
		t.Fatal("unknown marker type was accepted")
	}
	if got := Types(); !reflect.DeepEqual(got, markerTypes) {
		t.Fatalf("types = %#v", got)
	}
}
