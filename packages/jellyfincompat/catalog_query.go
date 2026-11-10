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

var errInvalidListQuery = errors.New("sortOrder, years, genres, nameStartsWith or sortBy is invalid")

type listQuery struct {
	genres     []string
	years      []string
	prefix     string
	sort       []string
	descending bool
}

func parseListQuery(values url.Values) (listQuery, error) {
	query := listQuery{prefix: strings.ToLower(Query(values, "nameStartsWith"))}
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
	if text := Query(values, "years"); text != "" {
		query.years = strings.Split(text, ",")
		if len(query.years) > maximumListYears || slices.ContainsFunc(query.years, func(year string) bool {
			return len(year) != 4 || strings.Trim(year, "0123456789") != ""
		}) {
			return listQuery{}, errInvalidListQuery
		}
	}
	if text := Query(values, "genres"); text != "" {
		query.genres = strings.Split(strings.ToLower(text), "|")
		if len(query.genres) > maximumListNames || slices.ContainsFunc(query.genres, func(genre string) bool { return len(genre) > maximumListText }) {
			return listQuery{}, errInvalidListQuery
		}
	}
	return query, nil
}

func (query listQuery) filters() bool {
	return len(query.genres) > 0 || len(query.years) > 0 || query.prefix != ""
}

func (query listQuery) matches(entry catalogEntry) bool {
	if query.prefix != "" && !strings.HasPrefix(entry.lower, query.prefix) {
		return false
	}
	if len(query.years) > 0 && !slices.Contains(query.years, entry.year()) {
		return false
	}
	if len(query.genres) == 0 {
		return true
	}
	for genre := range strings.SplitSeq(strings.ToLower(entry.genres()), " · ") {
		if genre != "" && slices.Contains(query.genres, genre) {
			return true
		}
	}
	return false
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
