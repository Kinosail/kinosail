package jellyfincompat

import (
	"errors"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const defaultLatestLimit = 20

var errInvalidLimit = errors.New("limit must be a 32-bit integer")

type entryFilter struct {
	ids      map[string]bool
	includes string
	types    []string
	search   string
}

func requestedLimit(values url.Values) (int, bool, error) {
	text := Query(values, "limit")
	if text == "" {
		return 0, false, nil
	}
	limit, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return 0, false, errInvalidLimit
	}
	return int(limit), true, nil
}

func itemsLimit(values url.Values) (int, error) {
	limit, present, err := requestedLimit(values)
	if !present || limit < 0 {
		return math.MaxInt, err
	}
	return limit, nil
}

func latestLimit(values url.Values) (int, error) {
	limit, present, err := requestedLimit(values)
	switch {
	case !present:
		return defaultLatestLimit, err
	case limit < 0:
		return 1, nil
	default:
		return limit, nil
	}
}

func newEntryFilter(values url.Values, ids map[string]bool) entryFilter {
	includes := Query(values, "includeItemTypes")
	return entryFilter{ids: ids, includes: includes, types: strings.Split(includes, ","), search: strings.ToLower(Query(values, "searchTerm"))}
}

func (filter entryFilter) all() bool {
	return len(filter.ids) == 0 && filter.includes == "" && filter.search == ""
}

func (filter entryFilter) matches(entry catalogEntry) bool {
	return (len(filter.ids) == 0 || filter.ids[entry.raw]) &&
		(filter.includes == "" || includesType(filter.types, entry.kind)) &&
		(filter.search == "" || strings.Contains(entry.lower, filter.search))
}

func (filter entryFilter) page(entries []catalogEntry, start, limit int) ([]catalogEntry, int, int) {
	if filter.all() {
		start = min(start, len(entries))
		return entries[start : start+min(limit, len(entries)-start)], start, len(entries)
	}
	page, total := make([]catalogEntry, 0), 0
	for _, entry := range entries {
		if filter.matches(entry) {
			if total >= start && len(page) < limit {
				page = append(page, entry)
			}
			total++
		}
	}
	return page, min(start, total), total
}

func (filter entryFilter) first(entries []catalogEntry, limit int) []catalogEntry {
	if filter.all() {
		return entries[:min(limit, len(entries))]
	}
	page := make([]catalogEntry, 0)
	for _, entry := range entries {
		if len(page) == limit {
			break
		}
		if filter.matches(entry) {
			page = append(page, entry)
		}
	}
	return page
}

func newestFirst(entries []catalogEntry) []catalogEntry {
	if entries == nil {
		return nil
	}
	ordered := slices.Clone(entries)
	slices.SortStableFunc(ordered, func(left, right catalogEntry) int {
		if left.item == nil || right.item == nil {
			return undated(left) - undated(right)
		}
		return right.item.Added.Compare(left.item.Added)
	})
	return ordered
}

func undated(entry catalogEntry) int {
	if entry.item == nil {
		return 1
	}
	return 0
}
