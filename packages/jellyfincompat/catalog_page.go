package jellyfincompat

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	maximumPageSize    = 1000
	defaultLatestLimit = 20
)

type entryFilter struct {
	ids      map[string]bool
	includes string
	types    []string
	search   string
}

func pageLimit(values url.Values, fallback int) int {
	limit, err := strconv.Atoi(Query(values, "limit"))
	switch {
	case errors.Is(err, strconv.ErrRange) && limit > 0:
		return maximumPageSize
	case err != nil || limit < 0:
		return fallback
	default:
		return min(limit, maximumPageSize)
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
	page, total := make([]catalogEntry, 0, min(limit, len(entries))), 0
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
	page := make([]catalogEntry, 0, min(limit, len(entries)))
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
