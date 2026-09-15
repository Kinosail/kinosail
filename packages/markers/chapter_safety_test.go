package markers

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestChapterDetectionAbstainsFromStoryAndAmbiguousLabels(t *testing.T) {
	t.Parallel()
	for _, title := range []string{
		"Epilogue", "Post-credits scene", "Mid Credits Scene", "After the credits",
		"Introduction to the suspect", "An introspective moment", "Recapturing the ship",
		"Commercial district", "End credits and final scene", "Intro / Recap",
		"Not credits", "The theme song returns", "", strings.Repeat("x", 513),
	} {
		t.Run(title, func(t *testing.T) {
			chapters := []metadata.Chapter{{Title: title, Start: 10, End: 40}}
			before := append([]metadata.Chapter(nil), chapters...)
			if got := DetectPlaybackMarkers(chapters); len(got) != 0 {
				t.Fatalf("story chapter became skippable: %#v", got)
			}
			if !reflect.DeepEqual(chapters, before) {
				t.Fatal("chapter detection changed source chapters")
			}
		})
	}
}

func TestChapterDetectionAcceptsOnlyBoundedValidRanges(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]float64{{-1, 10}, {10, 10}, {11, 10}, {math.NaN(), 10}, {0, math.NaN()}, {0, math.Inf(1)}, {math.Inf(-1), 10}} {
		if got := DetectPlaybackMarkers([]metadata.Chapter{{Title: "Intro", Start: bounds[0], End: bounds[1]}}); len(got) != 0 {
			t.Fatalf("invalid range became skippable: %#v", got)
		}
	}
	chapters := make([]metadata.Chapter, 4097)
	for index := range chapters {
		chapters[index] = metadata.Chapter{Title: "Intro", Start: float64(index), End: float64(index + 1)}
	}
	if got := DetectPlaybackMarkers(chapters); len(got) != 0 {
		t.Fatal("oversized chapter list was accepted")
	}
}

func TestChapterDetectionNormalizesExplicitLabels(t *testing.T) {
	t.Parallel()
	for title, kind := range map[string]string{
		"  OPENING\u00a0CREDITS  ": "intro", "Opening Titles": "intro",
		"End-Credits": "credits", "[Outro]": "outro", "Commercials": "commercial",
		"Previously on Kinosail": "recap", "Recap": "recap",
	} {
		got := DetectPlaybackMarkers([]metadata.Chapter{{Title: title, Start: 1, End: 20}})
		if len(got) != 1 || got[0].Type != kind || got[0].Source != "chapter" || got[0].Start != 1 || got[0].End != 20 {
			t.Fatalf("%q markers = %#v", title, got)
		}
	}
}
