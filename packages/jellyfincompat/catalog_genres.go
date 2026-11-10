package jellyfincompat

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type GenreRecord struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
	Type string `json:"Type"`
}

type Filters struct {
	Genres          []string `json:"Genres"`
	Tags            []string `json:"Tags"`
	OfficialRatings []string `json:"OfficialRatings"`
	Years           []int    `json:"Years"`
}

func genreID(name string) string {
	sum := sha256.Sum256([]byte("genre:" + strings.ToLower(name)))
	return hex.EncodeToString(sum[:16])
}

func (catalog Catalog) Genres(values url.Values) (Page, error) {
	limit, err := itemsLimit(values)
	search := strings.ToLower(Query(values, "searchTerm"))
	if err != nil || len(search) > maximumListText {
		return Page{}, errors.Join(err, errInvalidListQuery)
	}
	genres := make([]GenreRecord, 0)
	for _, name := range catalog.Filters(values).Genres {
		if search == "" || strings.Contains(strings.ToLower(name), search) {
			genres = append(genres, GenreRecord{ID: genreID(name), Name: name, Type: "Genre"})
		}
	}
	start := min(max(0, Int(values, "startIndex")), len(genres))
	return Result(genres[start:start+min(limit, len(genres)-start)], start, len(genres)), nil
}

func (catalog Catalog) Filters(values url.Values) Filters {
	filters := Filters{Genres: []string{}, Tags: []string{}, OfficialRatings: []string{}, Years: []int{}}
	seen := map[string]bool{}
	for _, entry := range catalog.under(Query(values, "parentId"), Query(values, "includeItemTypes")) {
		if year, err := strconv.Atoi(entry.year()); err == nil && !slices.Contains(filters.Years, year) {
			filters.Years = append(filters.Years, year)
		}
		for genre := range strings.SplitSeq(entry.genres(), " · ") {
			if key := strings.ToLower(genre); key != "" && !seen[key] {
				seen[key] = true
				filters.Genres = append(filters.Genres, genre)
			}
		}
	}
	slices.Sort(filters.Years)
	slices.SortFunc(filters.Genres, func(left, right string) int { return strings.Compare(strings.ToLower(left), strings.ToLower(right)) })
	return filters
}
