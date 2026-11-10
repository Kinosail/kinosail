package jellyfincompat

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	maximumListNames = 16
	maximumListYears = 32
	maximumListText  = 64
	maximumSortText  = 256
)

var errInvalidListQuery = errors.New("sortOrder, years, genres, genreIds, nameStartsWith, sortBy, searchTerm, filters, isPlayed or isFavorite is invalid")

type listQuery struct {
	genres     []string
	genreIDs   []string
	years      []string
	prefix     string
	sort       []string
	descending bool
	state      stateQuery
}

func parseListQuery(values url.Values) (listQuery, error) {
	state, err := parseStateQuery(values)
	if err != nil {
		return listQuery{}, err
	}
	query := listQuery{prefix: strings.ToLower(Query(values, "nameStartsWith")), state: state}
	order, sortBy := strings.ToLower(Query(values, "sortOrder")), Query(values, "sortBy")
	if len(query.prefix) > maximumListText || len(sortBy) > maximumSortText || (order != "" && order != "ascending" && order != "descending") {
		return listQuery{}, errInvalidListQuery
	}
	query.descending = order == "descending"
	for key := range strings.SplitSeq(strings.ToLower(sortBy), ",") {
		switch strings.TrimSpace(key) {
		case "sortname", "name":
			query.sort = append(query.sort, "name")
		case "datecreated":
			query.sort = append(query.sort, "added")
		case "productionyear", "premieredate":
			query.sort = append(query.sort, "year")
		}
	}
	years, validYears := boundedList(Query(values, "years"), ",", maximumListYears, func(year string) bool {
		return len(year) == 4 && strings.Trim(year, "0123456789") == ""
	})
	genres, validGenres := boundedList(strings.ToLower(Query(values, "genres")), "|", maximumListNames, func(genre string) bool { return len(genre) <= maximumListText })
	ids, validIDs := boundedList(Query(values, "genreIds"), "|,", maximumListNames, func(id string) bool {
		return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == ""
	})
	if !validYears || !validGenres || !validIDs {
		return listQuery{}, errInvalidListQuery
	}
	query.years, query.genres, query.genreIDs = years, genres, ids
	return query, nil
}

func boundedList(text, separators string, limit int, valid func(string) bool) ([]string, bool) {
	parts := strings.FieldsFunc(text, func(separator rune) bool { return strings.ContainsRune(separators, separator) })
	return parts, len(parts) <= limit && !slices.ContainsFunc(parts, func(part string) bool { return !valid(part) })
}

func (query listQuery) filters() bool {
	return len(query.genres) > 0 || len(query.genreIDs) > 0 || len(query.years) > 0 || query.prefix != "" || query.state.filters()
}

func (query listQuery) matches(entry catalogEntry) bool {
	if query.prefix != "" && !strings.HasPrefix(entry.lower, query.prefix) {
		return false
	}
	if len(query.years) > 0 && !slices.Contains(query.years, entry.year()) {
		return false
	}
	names, ids := len(query.genres) == 0, len(query.genreIDs) == 0
	for genre := range strings.SplitSeq(strings.ToLower(entry.genres()), " · ") {
		if genre != "" && (!names || !ids) {
			names = names || slices.Contains(query.genres, genre)
			ids = ids || slices.Contains(query.genreIDs, GenreID(genre))
		}
	}
	return names && ids && query.state.matches(entry)
}

func (query listQuery) order(entries []catalogEntry) []catalogEntry {
	if len(query.sort) == 0 || len(entries) < 2 {
		return entries
	}
	ordered := slices.Clone(entries)
	slices.SortStableFunc(ordered, func(left, right catalogEntry) int {
		for _, key := range query.sort {
			result := 0
			switch key {
			case "name":
				result = strings.Compare(left.sortName(), right.sortName())
			case "added":
				result = left.added().Compare(right.added())
			case "year":
				result = strings.Compare(left.year(), right.year())
			}
			if result != 0 {
				if query.descending {
					return -result
				}
				return result
			}
		}
		return 0
	})
	return ordered
}

func (entry catalogEntry) sortName() string {
	if entry.item != nil && entry.item.SortTitle != "" {
		return strings.ToLower(entry.item.SortTitle)
	}
	return entry.lower
}

func (entry catalogEntry) year() string {
	switch {
	case entry.item != nil:
		return entry.item.Year
	case entry.show != nil && entry.season == 0 && entry.kind == "Series":
		return entry.show.Year
	}
	return ""
}

func (entry catalogEntry) genres() string {
	switch {
	case entry.item != nil:
		return entry.item.Genres
	case entry.show != nil && entry.kind == "Series":
		return entry.show.Genres
	}
	return ""
}

func (entry catalogEntry) added() time.Time {
	if entry.item != nil {
		return entry.item.Added
	}
	var newest time.Time
	if entry.show != nil {
		for position := range entry.show.Episodes {
			if episode := &entry.show.Episodes[position]; (entry.kind == "Series" || episode.Season == entry.season) && episode.Added.After(newest) {
				newest = episode.Added
			}
		}
	}
	return newest
}
