package metadata

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestResolveTMDBMovieRejectsSearchAndArtworkIdentityFailures(t *testing.T) {
	t.Parallel()
	providerCalls := 0
	invalid := func(context.Context, string, any) error { providerCalls++; return nil }
	if _, err := resolveTMDBMovie(t.Context(), library.Item{ID: "../movie", Title: "Movie"}, "https://example.com", invalid, func(context.Context, int) string { providerCalls++; return "" }, func(string) string { providerCalls++; return "" }); err == nil || providerCalls != 0 {
		t.Fatalf("invalid movie identity reached provider or artwork: %d calls, %v", providerCalls, err)
	}
	failing := func(context.Context, string, any) error { return errors.New("offline") }
	if _, err := resolveTMDBMovie(t.Context(), library.Item{Title: "Movie"}, "https://example.com", failing, func(context.Context, int) string { return "" }, func(string) string { return "" }); err == nil {
		t.Fatal("movie search failure was ignored")
	}
	fetch := candidateFetch(TMDBCandidate{ID: 1, Title: "Movie", PosterPath: "/poster.jpg"})
	if _, err := resolveTMDBMovie(t.Context(), library.Item{ID: "../movie", Title: "Movie"}, "https://example.com", fetch, func(context.Context, int) string { return "" }, func(string) string { return "art" }); err == nil {
		t.Fatal("invalid movie artwork identity was accepted")
	}
}

func TestResolveTMDBShowRejectsProviderAndArtworkFailures(t *testing.T) { //nolint:cyclop // Show lookup, details, and both artwork targets fail independently.
	t.Parallel()
	providerCalls := 0
	if _, err := resolveTMDBShow(t.Context(), library.Item{ID: "../episode", Show: "Show"}, "https://example.com", func(context.Context, string, any) error { providerCalls++; return nil }, func(context.Context, library.Item, int, string, string, *Record, *string) { providerCalls++ }, func(string) string { providerCalls++; return "" }); err == nil || providerCalls != 0 {
		t.Fatalf("invalid show identity reached provider or artwork: %d calls, %v", providerCalls, err)
	}
	noopDetails := func(context.Context, library.Item, int, string, string, *Record, *string) {}
	if _, err := resolveTMDBShow(t.Context(), library.Item{Show: "Show"}, "https://example.com", func(context.Context, string, any) error { return errors.New("offline") }, noopDetails, func(string) string { return "art" }); err == nil {
		t.Fatal("show search failure was ignored")
	}
	valid := candidateFetch(TMDBCandidate{ID: 1, Name: "Show"})
	badDetails := func(_ context.Context, _ library.Item, _ int, _, _ string, _ *Record, poster *string) {
		*poster = "/../bad"
	}
	if _, err := resolveTMDBShow(t.Context(), library.Item{ID: "episode", Show: "Show"}, "https://example.com", valid, badDetails, func(string) string { return "art" }); err == nil {
		t.Fatal("invalid episode details were accepted")
	}
	withShowPoster := candidateFetch(TMDBCandidate{ID: 1, Name: "Show", PosterPath: "/show.jpg"})
	if _, err := resolveTMDBShow(t.Context(), library.Item{Show: "Show"}, "https://example.com", withShowPoster, noopDetails, func(string) string { return "" }); err == nil {
		t.Fatal("empty show artwork target was accepted")
	}
	episodeDetails := func(_ context.Context, _ library.Item, _ int, _, _ string, _ *Record, poster *string) {
		*poster = "/episode.jpg"
	}
	if _, err := resolveTMDBShow(t.Context(), library.Item{ID: "../episode", Show: "Show"}, "https://example.com", valid, episodeDetails, func(string) string { return "art" }); err == nil {
		t.Fatal("invalid episode artwork identity was accepted")
	}
	if _, err := resolveTMDBShow(t.Context(), library.Item{ID: "episode", Show: "Show"}, "https://example.com", valid, episodeDetails, func(string) string { return strings.Repeat("x", 4097) }); err == nil {
		t.Fatal("oversized episode artwork target was accepted")
	}
}

func TestFindTMDBCandidateReturnsSearchAndFallbackErrors(t *testing.T) { //nolint:cyclop // Each provider-stage error is translated to the shared contract.
	t.Parallel()
	identity := library.Item{Title: "Show", Year: "2020"}
	if _, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", identity, nil, func(context.Context, string, any) error { return errors.New("offline") }); err == nil {
		t.Fatal("candidate search failure was ignored")
	}
	searches := 0
	fallbackFailure := func(_ context.Context, _ string, target any) error {
		searches++
		if searches == 1 {
			*target.(*TMDBCandidates) = TMDBCandidates{}
			return nil
		}
		return errors.New("offline")
	}
	if _, err := FindTMDBCandidate(t.Context(), "https://example.com", "tv", identity, nil, fallbackFailure); err == nil {
		t.Fatal("TV fallback failure was ignored")
	}
	if _, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", identity, map[string]string{"imdb": "tt1234567"}, func(context.Context, string, any) error { return errors.New("offline") }); err == nil {
		t.Fatal("IMDb lookup failure was ignored")
	}
}

func TestTMDBIMDbAndTVFallbackEdgePolicies(t *testing.T) { //nolint:cyclop // Empty ID matches and conservative TV fallback behavior are explicit.
	t.Parallel()
	empty, err := findTMDBByIMDb(t.Context(), "https://example.com", "movie", map[string]string{"imdb": "tt1234567"}, func(_ context.Context, _ string, target any) error {
		*target.(*TMDBCandidates) = TMDBCandidates{}
		return nil
	})
	if err != nil || len(empty.Results) != 0 {
		t.Fatalf("empty IMDb match = %#v, %v", empty, err)
	}
	var endpoint string
	withoutRegion, err := searchTMDBTVFallback(t.Context(), "https://example.com", "tv", "Show", "2020", TMDBCandidates{}, func(_ context.Context, value string, target any) error {
		endpoint = value
		*target.(*TMDBCandidates) = TMDBCandidates{Results: []TMDBCandidate{{ID: 1, Name: "Show"}}}
		return nil
	})
	parsed, parseErr := url.Parse(endpoint)
	if err != nil || parseErr != nil || len(withoutRegion.Results) != 1 || parsed.Query().Get("first_air_date_year") != "" {
		t.Fatalf("fallback = %#v, endpoint=%q, err=%v, parse=%v", withoutRegion, endpoint, err, parseErr)
	}
	if _, err := searchTMDBTVFallback(t.Context(), "https://example.com", "tv", "Show", "", TMDBCandidates{}, func(context.Context, string, any) error { return errors.New("offline") }); err == nil {
		t.Fatal("TV fallback provider failure was ignored")
	}
	if _, err := searchTMDBTVFallback(t.Context(), "https://example.com", "tv", "Show", "", TMDBCandidates{}, candidateFetch(TMDBCandidate{ID: 1}, TMDBCandidate{ID: 2})); err == nil {
		t.Fatal("ambiguous TV fallback was accepted")
	}
}

func candidateFetch(candidates ...TMDBCandidate) func(context.Context, string, any) error {
	return func(_ context.Context, _ string, target any) error {
		*target.(*TMDBCandidates) = TMDBCandidates{Results: candidates}
		return nil
	}
}
