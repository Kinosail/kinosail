package catalog

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const maximumFacetValues = 100

type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Facets struct {
	Genres  []Facet `json:"genres"`
	Decades []Facet `json:"decades"`
	Ratings []Facet `json:"ratings"`
}

type Filters struct {
	Genre   string `json:"genre"`
	Decade  string `json:"decade"`
	Rating  string `json:"rating"`
	Watched string `json:"watched"`
}

func (filters Filters) Active() bool {
	return filters != Filters{}
}

func (filters Filters) intrinsic() bool {
	return filters.Genre != "" || filters.Decade != "" || filters.Rating != ""
}

func parseFilters(values url.Values, view string) (Filters, error) {
	var filters Filters
	var err error
	if filters.Genre, err = filterValue(values, "genre", 64); err != nil {
		return Filters{}, err
	}
	if filters.Rating, err = filterValue(values, "rating", 16); err != nil {
		return Filters{}, err
	}
	if filters.Decade, err = singleValue(values, "decade", 4); err != nil || filters.Decade != "" && !validDecade(filters.Decade) {
		return Filters{}, fmt.Errorf("%w: library decade is invalid", ErrInvalidBrowse)
	}
	if filters.Watched, err = singleValue(values, "watched", 16); err != nil || !oneOf(filters.Watched, "", "watched", "unwatched") {
		return Filters{}, fmt.Errorf("%w: library watched filter is invalid", ErrInvalidBrowse)
	}
	if filters.Active() && oneOf(view, "history", "collections", "playlists", "photos") {
		return Filters{}, fmt.Errorf("%w: library filters do not apply to this view", ErrInvalidBrowse)
	}
	return filters, nil
}

func filterValue(values url.Values, name string, maximum int) (string, error) {
	value, err := singleValue(values, name, maximum)
	if err != nil {
		return "", err
	}
	for _, character := range value {
		if character < ' ' || character == 0x7f {
			return "", fmt.Errorf("%w: library %s is invalid", ErrInvalidBrowse, name)
		}
	}
	return strings.TrimSpace(value), nil
}

func validDecade(value string) bool {
	if len(value) != 4 || value[3] != '0' {
		return false
	}
	for _, character := range []byte(value) {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value >= "1880" && value <= "2090"
}

func itemGenres(item *library.Item) string {
	if item.Show != "" && item.ShowGenres != "" {
		return item.ShowGenres
	}
	return item.Genres
}

func itemDecade(item *library.Item) string {
	year := item.Year
	if item.Show != "" && item.ShowYear != "" {
		year = item.ShowYear
	}
	if len(year) < 4 || !validDecade(year[:3]+"0") {
		return ""
	}
	return year[:3] + "0"
}

func (filters Filters) matches(item *library.Item) bool {
	if filters.Decade != "" && itemDecade(item) != filters.Decade {
		return false
	}
	if filters.Rating != "" && item.Rating != filters.Rating {
		return false
	}
	if filters.Genre == "" {
		return true
	}
	for _, genre := range strings.Split(itemGenres(item), " · ") {
		if genre == filters.Genre {
			return true
		}
	}
	return false
}

func (filters Filters) matchesWatched(watched bool) bool {
	return filters.Watched == "" || watched == (filters.Watched == "watched")
}

type facetCounter struct {
	genres, decades, ratings map[string]int
	shows                    map[string]struct{}
}

func newFacetCounter() *facetCounter {
	return &facetCounter{genres: map[string]int{}, decades: map[string]int{}, ratings: map[string]int{}, shows: map[string]struct{}{}}
}

func (counter *facetCounter) count(counts map[string]int, facet, show, value string) {
	if value == "" {
		return
	}
	if show != "" {
		key := facet + show + "\x00" + value
		if _, counted := counter.shows[key]; counted {
			return
		}
		counter.shows[key] = struct{}{}
	}
	counts[value]++
}

func (counter *facetCounter) add(item *library.Item) {
	show := ""
	if item.Kind == "video" && item.Show != "" {
		show = strings.ToLower(item.Show)
	}
	if genres := itemGenres(item); genres != "" {
		for _, genre := range strings.Split(genres, " · ") {
			counter.count(counter.genres, "g", show, genre)
		}
	}
	counter.count(counter.decades, "d", show, itemDecade(item))
	counter.count(counter.ratings, "r", show, item.Rating)
}

func (counter *facetCounter) facets() Facets {
	return Facets{Genres: facetValues(counter.genres, false), Decades: facetValues(counter.decades, true), Ratings: facetValues(counter.ratings, false)}
}

func facetValues(counts map[string]int, descending bool) []Facet {
	values := make([]Facet, 0, len(counts))
	for value, count := range counts {
		values = append(values, Facet{Value: value, Count: count})
	}
	if len(values) > maximumFacetValues {
		sort.Slice(values, func(left, right int) bool {
			return values[left].Count > values[right].Count || values[left].Count == values[right].Count && values[left].Value < values[right].Value
		})
		values = values[:maximumFacetValues]
	}
	sort.Slice(values, func(left, right int) bool {
		return (values[left].Value < values[right].Value) != descending
	})
	return values
}

func itemFacets(items []*library.Item) Facets {
	counter := newFacetCounter()
	for _, item := range items {
		if item != nil {
			counter.add(item)
		}
	}
	return counter.facets()
}
