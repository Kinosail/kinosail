package markers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestAnalyzerGroupingAndUnavailableMediaBoundaries(t *testing.T) {
	t.Parallel()
	episode := library.Item{ID: "episode", Kind: "video", Library: "TV", Show: "Show", Season: 1, Episode: 1, Size: 1, Added: time.Unix(1, 0)}
	movie := library.Item{ID: "movie", Kind: "video", Library: "Movies", Size: 2, Added: time.Unix(2, 0)}
	groups := groupMarkerItems([]library.Item{{ID: "book", Kind: "book"}, episode, movie})
	if len(groups.seasons) != 1 || len(groups.movies) != 1 {
		t.Fatalf("marker groups = %#v", groups)
	}
	analyzer := NewAnalyzer(Config{})
	if media := analyzer.inspect(t.Context(), movie); media.Duration != 0 || media.Markers != nil {
		t.Fatalf("nil probe media = %#v", media)
	}
	if err := analyzer.analyzeMovieOpenings(t.Context(), []library.Item{movie}); err == nil {
		t.Fatal("unavailable analysis succeeded")
	}
	if err := analyzer.analyzeEpisodeGroups(t.Context(), map[string][]library.Item{"single": {episode}}); err != nil {
		t.Fatal(err)
	}
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{} })
	if err := analyzer.analyzeEpisodeGroups(t.Context(), map[string][]library.Item{"season": {episode, episode}}); err == nil {
		t.Fatal("unavailable analysis succeeded")
	}
	if err := analyzer.analyzeSeason(t.Context(), []library.Item{episode, episode}); err == nil {
		t.Fatal("unavailable analysis succeeded")
	}
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 100} })
	analyzer.extract = func(context.Context, library.Item, string, float64, float64) ([]uint32, error) {
		return nil, errors.New("audio unavailable")
	}
	analyzer.extractVisual = func(context.Context, library.Item, string, float64, float64) ([]uint64, error) {
		return nil, errors.New("visual unavailable")
	}
	if err := analyzer.analyzeSeason(t.Context(), []library.Item{episode, episode}); err == nil {
		t.Fatal("unavailable analysis succeeded")
	}
	if err := analyzer.analyzeMovieOpenings(t.Context(), []library.Item{movie}); err == nil {
		t.Fatal("unavailable analysis succeeded")
	}
}

func TestAnalyzerCreditsSkipsProtectedItemsAndStoresSuccess(t *testing.T) {
	t.Parallel()
	items := []library.Item{
		{ID: "book", Kind: "book"},
		{ID: "zero", Kind: "video"},
		{ID: "chapter", Kind: "video", Size: 1, Added: time.Unix(1, 0)},
		{ID: "manual", Kind: "video", Size: 2, Added: time.Unix(2, 0)},
		{ID: "failed", Kind: "video", Size: 3, Added: time.Unix(3, 0)},
		{ID: "stored", Kind: "video", Size: 4, Added: time.Unix(4, 0)},
	}
	analyzer := NewAnalyzer(Config{})
	analyzer.SetProbe(func(_ context.Context, item library.Item) Media {
		switch item.ID {
		case "zero":
			return Media{}
		case "chapter":
			return Media{Duration: 200, Markers: []Marker{{Type: "credits", Source: "chapter"}}}
		case "manual":
			return Media{Duration: 200, Markers: []Marker{{Type: "credits", Source: "manual"}}}
		default:
			return Media{Duration: 200}
		}
	})
	analyzer.extractCredits = func(_ context.Context, item library.Item, _, _ float64) ([]byte, error) {
		if item.ID == "failed" {
			return nil, errors.New("frames unavailable")
		}
		return nil, nil
	}
	if err := analyzer.analyzeCredits(t.Context(), items); err == nil {
		t.Fatal("unavailable duration was reported as successful")
	}
	if err := analyzer.analyzeCredits(t.Context(), items[2:]); err == nil {
		t.Fatal("failed extraction was reported as successful")
	}
	if _, found := analyzer.records["failed"]; found {
		t.Fatal("failed credit analysis changed state")
	}
	if err := analyzer.analyzeCredits(t.Context(), items[5:]); err != nil {
		t.Fatal(err)
	}
	if record, found := analyzer.records["stored"]; !found || record.Revision != mediaRevision(items[5]) {
		t.Fatalf("successful credit analysis record = %#v, %v", record, found)
	}
}

func TestAnalyzerVisualTailComparisonAndCandidateBoundaries(t *testing.T) {
	t.Parallel()
	shared := visualSequence(30, 9000)
	left := episodeFingerprint{duration: 300, tailOffset: 200, tailVisual: shared}
	right := episodeFingerprint{duration: 300, tailOffset: 210, tailVisual: append([]uint64(nil), shared...)}
	analyzer := NewAnalyzer(Config{})
	analyzer.comparePair(&left, &right)
	if len(left.outroVisual.ranges) == 0 || len(right.outroVisual.ranges) == 0 {
		t.Fatalf("visual tail candidates = %#v / %#v", left.outroVisual, right.outroVisual)
	}
	rejected := markerCandidate{}
	acceptCandidate(&rejected, timeRange{Start: 1, End: 2}, 100, false)
	if len(rejected.ranges) != 0 {
		t.Fatal("short candidate was accepted")
	}
}

func TestChangedSeasonInvalidatesSiblingEpisodes(t *testing.T) {
	t.Parallel()
	items := []library.Item{
		{ID: "one", Kind: "video", Library: "TV", Show: "Show", Season: 1, Size: 1, Added: time.Unix(1, 0)},
		{ID: "two", Kind: "video", Library: "TV", Show: "Show", Season: 1, Size: 2, Added: time.Unix(2, 0)},
	}
	analyzer := NewAnalyzer(Config{})
	analyzer.records["two"] = Record{Revision: mediaRevision(items[1]), DetectorVersion: DetectorVersion}
	changed := analyzer.changedItems(items)
	if len(changed) != 2 {
		t.Fatalf("changed season = %#v", changed)
	}
}
