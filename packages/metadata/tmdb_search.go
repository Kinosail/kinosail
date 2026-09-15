package metadata

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

var tvRegionSuffix = regexp.MustCompile(`(?i)\s+\([a-z]{2,3}\)$`)

// ResolveTMDBRecord applies Player's movie and series record-building policy.
func ResolveTMDBRecord(ctx context.Context, item library.Item, baseURL string, fetch func(context.Context, string, any) error, collection func(context.Context, int) string, details func(context.Context, library.Item, int, string, string, *Record, *string), artwork func(string) string) (Result, error) {
	if ctx == nil || fetch == nil || collection == nil || details == nil || artwork == nil {
		return Result{}, errors.New("metadata identity is invalid")
	}
	if item.Show == "" {
		return resolveTMDBMovie(ctx, item, baseURL, fetch, collection, artwork)
	}
	return resolveTMDBShow(ctx, item, baseURL, fetch, details, artwork)
}

func resolveTMDBMovie(ctx context.Context, item library.Item, baseURL string, fetch func(context.Context, string, any) error, collection func(context.Context, int) string, artwork func(string) string) (Result, error) {
	candidate, err := FindTMDBCandidate(ctx, baseURL, "movie", library.Item{Title: item.Title, Year: item.Year}, item.ProviderIDs, fetch)
	if err != nil {
		return Result{}, err
	}
	record := Record{Title: candidate.Title, Plot: candidate.Overview, Year: Year(candidate.ReleaseDate), Collection: collection(ctx, candidate.ID), ProviderIDs: map[string]string{"tmdb": strconv.Itoa(candidate.ID)}}
	if candidate.PosterPath != "" {
		if !validMetadataID(item.ID) {
			return Result{}, errors.New("metadata identity is invalid")
		}
		record.Artwork = artwork(item.ID)
	}
	if !Valid(record) || candidate.PosterPath != "" && record.Artwork == "" {
		return Result{}, errors.New("metadata provider returned invalid data")
	}
	result := Result{Record: record}
	if candidate.PosterPath != "" {
		result.Images = []Image{{Source: candidate.PosterPath, Target: record.Artwork}}
	}
	return enrichTMDBCast(ctx, baseURL, "movie", candidate.ID, fetch, artwork, result)
}

func resolveTMDBShow(ctx context.Context, item library.Item, baseURL string, fetch func(context.Context, string, any) error, details func(context.Context, library.Item, int, string, string, *Record, *string), artwork func(string) string) (Result, error) { //nolint:cyclop // Show identity, details, and two independent artwork targets form one result contract.
	showTitle := item.ShowTitle
	if showTitle == "" {
		showTitle = item.Show
	}
	show, err := FindTMDBCandidate(ctx, baseURL, "tv", library.Item{Title: showTitle, Year: item.ShowYear}, item.ShowProviderIDs, fetch)
	if err != nil {
		return Result{}, err
	}
	record := Record{Title: show.Name, Year: Year(show.FirstAirDate), ShowTitle: show.Name, ShowYear: Year(show.FirstAirDate), ShowPlot: show.Overview, ShowProviderIDs: map[string]string{"tmdb": strconv.Itoa(show.ID)}, ProviderIDs: TVDBProviderID(item)}
	poster := ""
	details(ctx, item, show.ID, show.Name, show.FirstAirDate, &record, &poster)
	if !ValidTMDBPath(poster) || !Valid(record) {
		return Result{}, errors.New("metadata provider returned invalid data")
	}
	result := Result{Record: record}
	if show.PosterPath != "" {
		result.Record.ShowArtwork = artwork("tv-" + strconv.Itoa(show.ID))
		if result.Record.ShowArtwork == "" || !Bounded(result.Record) {
			return Result{}, errors.New("metadata artwork path is invalid")
		}
		result.Images = append(result.Images, Image{Source: show.PosterPath, Target: result.Record.ShowArtwork, Show: true})
	}
	if poster != "" {
		if !validMetadataID(item.ID) {
			return Result{}, errors.New("metadata identity is invalid")
		}
		result.Record.Artwork = artwork(item.ID)
		if result.Record.Artwork == "" || !Bounded(result.Record) {
			return Result{}, errors.New("metadata artwork path is invalid")
		}
		result.Images = append(result.Images, Image{Source: poster, Target: result.Record.Artwork})
	}
	return enrichTMDBCast(ctx, baseURL, "tv", show.ID, fetch, artwork, result)
}

// FindTMDBCandidate applies Player's ID-first and conservative title matching policy.
func FindTMDBCandidate(ctx context.Context, baseURL, kind string, identity library.Item, providerIDs map[string]string, fetch func(context.Context, string, any) error) (TMDBCandidate, error) {
	query, searchYear, err := validTMDBIdentity(ctx, baseURL, kind, identity, fetch)
	if err != nil {
		return TMDBCandidate{}, err
	}
	candidates, err := findTMDBByIMDb(ctx, baseURL, kind, providerIDs, fetch)
	if err != nil {
		return TMDBCandidate{}, err
	}
	if len(candidates.Results) == 0 {
		candidates, err = searchTMDBCandidates(ctx, baseURL, kind, query, searchYear, fetch)
		if err != nil {
			return TMDBCandidate{}, errors.New("metadata was not found")
		}
	}
	candidates, err = searchTMDBTVFallback(ctx, baseURL, kind, query, searchYear, candidates, fetch)
	if err != nil {
		return TMDBCandidate{}, err
	}
	return validTMDBCandidate(candidates)
}

func validTMDBIdentity(ctx context.Context, baseURL, kind string, identity library.Item, fetch func(context.Context, string, any) error) (string, string, error) {
	query, year := MovieIdentity(identity)
	if ctx == nil || fetch == nil || !validProviderBaseURL(baseURL) || kind != "movie" && kind != "tv" || query == "" || len(query) > 200 || !validMetadataYear(year) || hasControlText(query) {
		return "", "", errors.New("metadata identity is invalid")
	}
	return query, year, nil
}

func findTMDBByIMDb(ctx context.Context, baseURL, kind string, providerIDs map[string]string, fetch func(context.Context, string, any) error) (TMDBCandidates, error) {
	imdb := providerIDs["imdb"]
	if !ValidIMDbID(imdb) {
		return TMDBCandidates{}, nil
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/find/" + url.PathEscape(imdb) + "?" + url.Values{"external_source": {"imdb_id"}}.Encode()
	var candidates TMDBCandidates
	if err := fetch(ctx, endpoint, &candidates); err != nil {
		return TMDBCandidates{}, errors.New("metadata was not found")
	}
	matches := map[string][]TMDBCandidate{"movie": candidates.MovieResults, "tv": candidates.TVResults}[kind]
	if len(matches) > 1 {
		return TMDBCandidates{}, errors.New("metadata match was ambiguous")
	}
	if len(matches) == 1 {
		candidates.Results = matches
		return candidates, nil
	}
	return TMDBCandidates{}, nil
}

func searchTMDBTVFallback(ctx context.Context, baseURL, kind, query, searchYear string, candidates TMDBCandidates, fetch func(context.Context, string, any) error) (TMDBCandidates, error) {
	if kind != "tv" || len(candidates.Results) != 0 {
		return candidates, nil
	}
	fallback := strings.TrimSpace(tvRegionSuffix.ReplaceAllString(query, ""))
	if fallback == query {
		searchYear = ""
	}
	candidates, err := searchTMDBCandidates(ctx, baseURL, kind, fallback, searchYear, fetch)
	if err != nil {
		return TMDBCandidates{}, errors.New("metadata was not found")
	}
	if len(candidates.Results) > 1 {
		return TMDBCandidates{}, errors.New("metadata match was ambiguous")
	}
	return candidates, nil
}

func validTMDBCandidate(candidates TMDBCandidates) (TMDBCandidate, error) {
	if len(candidates.Results) == 0 || len(candidates.Results) > 100 || candidates.Results[0].ID <= 0 || !ValidTMDBPath(candidates.Results[0].PosterPath) {
		return TMDBCandidate{}, errors.New("metadata was not found")
	}
	return candidates.Results[0], nil
}

func searchTMDBCandidates(ctx context.Context, baseURL, kind, query, searchYear string, fetch func(context.Context, string, any) error) (TMDBCandidates, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/search/" + kind
	values := url.Values{"query": {query}}
	if searchYear != "" {
		values.Set(map[string]string{"movie": "primary_release_year", "tv": "first_air_date_year"}[kind], searchYear)
	}
	var candidates TMDBCandidates
	err := fetch(ctx, endpoint+"?"+values.Encode(), &candidates)
	return candidates, err
}
