package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

var (
	movieYear      = regexp.MustCompile(`^(.*?)\s*[\[(]?((?:19|20)\d{2})[\])]?(?:\s+[\[{].*)?$`)
	imdbProviderID = regexp.MustCompile(`^tt[0-9]{7,9}$`)
)

type TMDBCandidate struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	ReleaseDate  string `json:"release_date"`
	FirstAirDate string `json:"first_air_date"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

type TMDBCandidates struct {
	Results      []TMDBCandidate `json:"results"`
	MovieResults []TMDBCandidate `json:"movie_results"`
	TVResults    []TMDBCandidate `json:"tv_results"`
}

// ValidTMDBPath validates one relative provider asset path.
func ValidTMDBPath(path string) bool {
	return path == "" || len(path) <= 2048 && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && pathpkg.Clean(path) == path && !strings.ContainsAny(path, `\?#%`) && !hasControlText(path)
}

func (client *TMDBClient) fetch(ctx context.Context, item library.Item) (TMDBMetadata, error) { //nolint:cyclop,gocognit // One provider operation validates ID-first matching and the search fallback together.
	title, year := MovieIdentity(item)
	if !validTMDBCacheID(item.ID) || title == "" || len(title) > 200 || !validMetadataYear(year) || hasControlText(title) {
		return TMDBMetadata{}, errors.New("TMDB identity is invalid")
	}
	var candidates TMDBCandidates
	if imdb := item.ProviderIDs["imdb"]; ValidIMDbID(imdb) {
		query := url.Values{"external_source": {"imdb_id"}}
		if err := client.get(ctx, "/find/"+url.PathEscape(imdb)+"?"+query.Encode(), &candidates); err != nil || len(candidates.MovieResults) > 1 {
			return TMDBMetadata{}, fmt.Errorf("no unique match for %q", imdb)
		}
		if len(candidates.MovieResults) == 1 {
			candidates.Results = candidates.MovieResults
		} else {
			candidates = TMDBCandidates{}
		}
	}
	if len(candidates.Results) == 0 {
		query := url.Values{"query": {title}, "include_adult": {"false"}}
		if year != "" {
			query.Set("primary_release_year", year)
		}
		if err := client.get(ctx, "/search/movie?"+query.Encode(), &candidates); err != nil {
			return TMDBMetadata{}, err
		}
	}
	if len(candidates.Results) == 0 || len(candidates.Results) > 100 || candidates.Results[0].ID <= 0 {
		return TMDBMetadata{}, fmt.Errorf("no match for %q", title)
	}
	var movie tmdbMovie
	if err := client.get(ctx, fmt.Sprintf("/movie/%d?append_to_response=credits", candidates.Results[0].ID), &movie); err != nil {
		return TMDBMetadata{}, err
	}
	metadata := metadataFor(movie)
	if !validTMDBText(metadata) || !ValidTMDBPath(movie.PosterPath) || !ValidTMDBCast(movie.Credits.Cast) {
		return TMDBMetadata{}, errors.New("TMDB returned invalid metadata")
	}
	metadata.TMDBID = candidates.Results[0].ID
	directory := filepath.Join(client.cacheDir, item.ID)
	if item.Artwork == "" {
		metadata.Poster, _ = client.download(ctx, movie.PosterPath, filepath.Join(directory, "poster"))
	}
	metadata.Cast = client.downloadCast(ctx, movie.Credits.Cast, directory)
	return metadata, nil
}

func ValidTMDBCast(cast []TMDBCastMember) bool {
	if len(cast) > 1000 {
		return false
	}
	for _, person := range cast {
		if len(person.Name) > 200 || len(person.Character) > 200 || hasControlText(person.Name+person.Character) || !ValidTMDBPath(person.ProfilePath) {
			return false
		}
	}
	return true
}

func validTMDBText(metadata TMDBMetadata) bool {
	return metadata.Title != "" && len(metadata.Title) <= 200 && validMetadataYear(metadata.Year) && len(metadata.Plot) <= 5000 && len(metadata.Genres) <= 500 && len(metadata.Director) <= 200 && !hasControlText(metadata.Title+metadata.Genres+metadata.Director) && !hasInvalidRecordText(metadata.Plot, true)
}

// MovieIdentity returns a normalized title and optional release year.
func MovieIdentity(item library.Item) (string, string) {
	title, year := strings.TrimSpace(item.Title), strings.TrimSpace(item.Year)
	if match := movieYear.FindStringSubmatch(title); match != nil {
		candidate := strings.TrimSpace(match[1])
		// A numeric title such as "1917" is a title, not a missing-title year.
		if candidate != "" {
			title = candidate
			if year == "" {
				year = match[2]
			}
		}
	}
	return title, year
}

// ValidIMDbID reports whether value is a bounded movie identity.
func ValidIMDbID(value string) bool { return imdbProviderID.MatchString(value) }
