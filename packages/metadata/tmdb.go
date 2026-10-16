package metadata

import "github.com/MikeO7/kinosail/packages/library"

type TMDBMetadata struct {
	Title, Year, Plot, Rating, Genres, Director, Poster string
	Cast                                                []library.Person
	TMDBID                                              int
	Version                                             int
}

type tmdbReleaseDates struct {
	Results []struct {
		Country string `json:"iso_3166_1"`
		Dates   []struct {
			Certification string `json:"certification"`
		} `json:"release_dates"`
	} `json:"results"`
}

type TMDBCastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
}
