package metadata

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFindTMDBCandidateUsesIDAndTitleFallbacks(t *testing.T) { //nolint:cyclop // One fake provider proves the ordered matching contract.
	t.Parallel()
	requests := make([]*url.URL, 0, 3)
	fetch := func(_ context.Context, endpoint string, target any) error {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return err
		}
		requests = append(requests, parsed)
		candidates := target.(*TMDBCandidates)
		switch parsed.Path {
		case "/v3/find/tt1234567":
			candidates.MovieResults = []TMDBCandidate{{ID: 7, Title: "ID match"}}
		case "/v3/search/movie":
			candidates.Results = []TMDBCandidate{{ID: 8, Title: "Title match", PosterPath: "/poster.jpg"}}
		case "/v3/search/tv":
			if parsed.Query().Get("query") == "Show" {
				candidates.Results = []TMDBCandidate{{ID: 9, Name: "Show"}}
			}
		}
		return nil
	}
	baseURL := "https://example.com/v3"
	matched, err := FindTMDBCandidate(t.Context(), baseURL, "movie", library.Item{Title: "Wrong"}, map[string]string{"imdb": "tt1234567"}, fetch)
	if err != nil || matched.ID != 7 || len(requests) != 1 || requests[0].Query().Get("external_source") != "imdb_id" {
		t.Fatalf("ID match = %#v, requests=%v, err=%v", matched, requests, err)
	}
	requests = nil
	matched, err = FindTMDBCandidate(t.Context(), baseURL, "movie", library.Item{Title: "Movie (2020)"}, nil, fetch)
	if err != nil || matched.ID != 8 || len(requests) != 1 || requests[0].Query().Get("query") != "Movie" || requests[0].Query().Get("primary_release_year") != "2020" {
		t.Fatalf("movie match = %#v, requests=%v, err=%v", matched, requests, err)
	}
	requests = nil
	matched, err = FindTMDBCandidate(t.Context(), baseURL, "tv", library.Item{Title: "Show (US)", Year: "2021"}, nil, fetch)
	if err != nil || matched.ID != 9 || len(requests) != 2 || requests[1].Query().Get("query") != "Show" || requests[1].Query().Get("first_air_date_year") != "2021" {
		t.Fatalf("TV fallback = %#v, requests=%v, err=%v", matched, requests, err)
	}
}

func TestFindTMDBCandidateRejectsInvalidBoundaries(t *testing.T) { //nolint:cyclop,funlen,staticcheck // Invalid local and provider inputs share one fail-closed contract, including an intentional nil context.
	t.Parallel()
	calls := 0
	fetch := func(ctx context.Context, _ string, target any) error {
		calls++
		target.(*TMDBCandidates).Results = []TMDBCandidate{{ID: 1}}
		return context.Cause(ctx)
	}
	valid := library.Item{Title: "Movie", Year: "2020"}
	for name, test := range map[string]func() error{
		"nil context": func() error {
			_, err := FindTMDBCandidate(nil, "https://example.com", "movie", valid, nil, fetch) //nolint:staticcheck // Intentional nil validates fail-closed behavior.
			return err
		},
		"nil fetch": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", valid, nil, nil)
			return err
		},
		"invalid URL": func() error {
			_, err := FindTMDBCandidate(t.Context(), "http://example.com", "movie", valid, nil, fetch)
			return err
		},
		"unknown kind": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "person", valid, nil, fetch)
			return err
		},
		"missing title": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", library.Item{Year: "2020"}, nil, fetch)
			return err
		},
		"long title": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", library.Item{Title: strings.Repeat("x", 201)}, nil, fetch)
			return err
		},
		"bad year": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", library.Item{Title: "Movie", Year: "20xx"}, nil, fetch)
			return err
		},
		"control": func() error {
			_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", library.Item{Title: "Movie\x00"}, nil, fetch)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := test(); err == nil {
				t.Fatal("invalid candidate boundary accepted")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid local input reached provider %d times", calls)
	}
	for name, candidates := range map[string]TMDBCandidates{
		"empty":        {},
		"invalid id":   {Results: []TMDBCandidate{{ID: 0}}},
		"invalid path": {Results: []TMDBCandidate{{ID: 1, PosterPath: "/../poster"}}},
		"too many":     {Results: make([]TMDBCandidate, 101)},
	} {
		t.Run(name, func(t *testing.T) {
			provider := func(_ context.Context, _ string, target any) error {
				*target.(*TMDBCandidates) = candidates
				return nil
			}
			if _, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", valid, nil, provider); err == nil {
				t.Fatal("invalid provider candidate accepted")
			}
		})
	}
}

func TestFindTMDBCandidateRejectsAmbiguousIMDbMatch(t *testing.T) {
	t.Parallel()
	fetch := func(_ context.Context, _ string, target any) error {
		target.(*TMDBCandidates).MovieResults = []TMDBCandidate{{ID: 1}, {ID: 2}}
		return nil
	}
	_, err := FindTMDBCandidate(t.Context(), "https://example.com", "movie", library.Item{Title: "Movie"}, map[string]string{"imdb": "tt1234567"}, fetch)
	if err == nil || err.Error() != "metadata match was ambiguous" {
		t.Fatalf("ambiguous match error = %v", err)
	}
}

func TestResolveTMDBRecordBuildsMovieAndEpisodeResults(t *testing.T) { //nolint:cyclop // One provider fixture covers both shared result shapes.
	t.Parallel()
	fetch := func(_ context.Context, endpoint string, target any) error {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return err
		}
		candidates, ok := target.(*TMDBCandidates)
		if !ok {
			return nil
		}
		switch parsed.Path {
		case "/search/movie":
			candidates.Results = []TMDBCandidate{{ID: 7, Title: "Movie", Overview: "Plot", ReleaseDate: "2020-01-02", PosterPath: "/movie.jpg"}}
		case "/search/tv":
			candidates.Results = []TMDBCandidate{{ID: 8, Name: "Show", Overview: "Show plot", FirstAirDate: "2019-01-02", PosterPath: "/show.jpg"}}
		}
		return nil
	}
	collection := func(context.Context, int) string { return "Collection" }
	details := func(_ context.Context, _ library.Item, id int, _, _ string, record *Record, poster *string) {
		if id != 8 {
			t.Errorf("show id = %d", id)
		}
		record.Title, record.Plot, *poster = "S01E02 · Episode", "Episode plot", "/episode.jpg"
	}
	artwork := func(id string) string { return "/cache/" + id + ".jpg" }
	movie, err := ResolveTMDBRecord(t.Context(), library.Item{ID: "movie", Title: "Movie", Year: "2020"}, "https://example.com", fetch, collection, details, artwork)
	if err != nil || movie.Record.Collection != "Collection" || movie.Record.ProviderIDs["tmdb"] != "7" || len(movie.Images) != 1 || movie.Images[0].Source != "/movie.jpg" {
		t.Fatalf("movie result = %#v, %v", movie, err)
	}
	episode, err := ResolveTMDBRecord(t.Context(), library.Item{ID: "episode", Show: "Show", ShowYear: "2019", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "123"}}, "https://example.com", fetch, collection, details, artwork)
	if err != nil || episode.Record.Title != "S01E02 · Episode" || episode.Record.ShowTitle != "Show" || episode.Record.ProviderIDs["tvdb"] != "123" || len(episode.Images) != 2 || !episode.Images[0].Show || episode.Images[1].Target != "/cache/episode.jpg" {
		t.Fatalf("episode result = %#v, %v", episode, err)
	}
}

func TestResolveTMDBRecordRejectsInvalidAdaptersAndProviderData(t *testing.T) { //nolint:staticcheck // An intentional nil context proves the adapter boundary fails closed.
	t.Parallel()
	item := library.Item{ID: "movie", Title: "Movie"}
	fetch := func(_ context.Context, _ string, target any) error {
		target.(*TMDBCandidates).Results = []TMDBCandidate{{ID: 1, Title: "Movie", PosterPath: "/poster.jpg"}}
		return nil
	}
	noopDetails := func(context.Context, library.Item, int, string, string, *Record, *string) {}
	for name, call := range map[string]func() error{
		"nil context": func() error {
			_, err := ResolveTMDBRecord(nil, item, "https://example.com", fetch, func(context.Context, int) string { return "" }, noopDetails, func(string) string { return "art" }) //nolint:staticcheck // Intentional nil validates fail-closed behavior.
			return err
		},
		"nil fetch": func() error {
			_, err := ResolveTMDBRecord(t.Context(), item, "https://example.com", nil, func(context.Context, int) string { return "" }, noopDetails, func(string) string { return "art" })
			return err
		},
		"empty artwork": func() error {
			_, err := ResolveTMDBRecord(t.Context(), item, "https://example.com", fetch, func(context.Context, int) string { return "" }, noopDetails, func(string) string { return "" })
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid TMDB record boundary accepted")
			}
		})
	}
}
