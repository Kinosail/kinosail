package metadata

import (
	"errors"
	pathpkg "path"
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

func ValidTMDBPath(path string) bool {
	return path == "" || len(path) <= 2048 && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && pathpkg.Clean(path) == path && !strings.ContainsAny(path, `\?#%`) && !hasControlText(path)
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
	return metadata.Title != "" && len(metadata.Title) <= 200 && validMetadataYear(metadata.Year) && len(metadata.Plot) <= 5000 && len(metadata.Genres) <= 500 && len(metadata.Director) <= 200 && (metadata.Rating == "" || validMovieRating(metadata.Rating)) && !hasControlText(metadata.Title+metadata.Genres+metadata.Director) && !hasInvalidRecordText(metadata.Plot, true)
}

func validMovieRating(value string) bool {
	switch value {
	case "G", "PG", "PG-13", "R", "NC-17", "NR", "UNRATED":
		return true
	}
	return false
}

func movieCertification(releases tmdbReleaseDates) (string, error) {
	if len(releases.Results) > 200 {
		return "", errors.New("TMDB returned too many release regions")
	}
	rating := ""
	for _, region := range releases.Results {
		if len(region.Dates) > 100 {
			return "", errors.New("TMDB returned too many release dates")
		}
		if region.Country != "US" {
			continue
		}
		for _, release := range region.Dates {
			var err error
			rating, err = addMovieCertification(rating, release.Certification)
			if err != nil {
				return "", err
			}
		}
	}
	return rating, nil
}

func addMovieCertification(current, value string) (string, error) {
	if len(value) > 32 {
		return "", errors.New("TMDB returned an invalid certification")
	}
	rating := strings.ToUpper(strings.TrimSpace(value))
	if rating != "" && !validMovieRating(rating) {
		return "", errors.New("TMDB returned an invalid certification")
	}
	if rating == "" {
		return current, nil
	}
	if current != "" && current != rating {
		return "", errors.New("TMDB returned conflicting certifications")
	}
	return rating, nil
}

func MovieIdentity(item library.Item) (string, string) {
	title, year := strings.TrimSpace(item.Title), strings.TrimSpace(item.Year)
	if match := movieYear.FindStringSubmatch(title); match != nil {
		candidate := strings.TrimSpace(match[1])
		if candidate != "" {
			title = candidate
			if year == "" {
				year = match[2]
			}
		}
	}
	return title, year
}

func ValidIMDbID(value string) bool { return imdbProviderID.MatchString(value) }
