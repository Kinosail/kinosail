package catalog

import (
	"bytes"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func sortReferences(items []*library.Item, order, query, locale string) { //nolint:cyclop,gocognit // All sort precedence stays explicit and deterministic.
	order = normalizeSort(order)
	titles := collate.New(language.Make(locale))
	if order == "title" && query == "" {
		sortTitles(items, titles)
		return
	}
	var ranks map[*library.Item]int
	if query != "" {
		normalized := searchText(query)
		ranks = make(map[*library.Item]int, len(items))
		for _, item := range items {
			ranks[item] = searchRankNormalized(*item, normalized)
		}
	}
	sort.Slice(items, func(left, right int) bool {
		if query != "" {
			leftRank, rightRank := ranks[items[left]], ranks[items[right]]
			if leftRank != rightRank {
				return leftRank < rightRank
			}
		}
		if order == "added" {
			return items[left].Added.After(items[right].Added) || items[left].Added.Equal(items[right].Added) && items[left].ID < items[right].ID
		}
		if order == "year" {
			return items[left].Year > items[right].Year || items[left].Year == items[right].Year && items[left].ID < items[right].ID
		}
		comparison := titles.CompareString(sortTitle(*items[left]), sortTitle(*items[right]))
		return comparison < 0 || comparison == 0 && items[left].ID < items[right].ID
	})
}

func sortTitles(items []*library.Item, titles *collate.Collator) {
	var buffer collate.Buffer
	keyed := make([]sortReference, len(items))
	for position, item := range items {
		keyed[position] = sortReference{item, titles.KeyFromString(&buffer, sortKey(*item))}
	}
	sort.Slice(keyed, func(left, right int) bool {
		comparison := bytes.Compare(keyed[left].key, keyed[right].key)
		return comparison < 0 || comparison == 0 && keyed[left].item.ID < keyed[right].item.ID
	})
	for position := range items {
		items[position] = keyed[position].item
	}
}

func normalizeSort(order string) string {
	if order == "added" || order == "year" {
		return order
	}
	return "title"
}

func singleValue(values url.Values, name string, maximum int) (string, error) {
	selected, found := values[name]
	if !found {
		return "", nil
	}
	if len(selected) != 1 || len(selected[0]) > maximum || !utf8.ValidString(selected[0]) {
		return "", fmt.Errorf("%w: library %s is invalid", ErrInvalidBrowse, name)
	}
	return selected[0], nil
}

// SingleValue reads one bounded UTF-8 query value.
func SingleValue(values url.Values, name string, maximum int) (string, error) {
	return singleValue(values, name, maximum)
}

func browseInteger(values url.Values, name string, fallback, minimum, maximum int) (int, error) {
	selected, found := values[name]
	if !found {
		return fallback, nil
	}
	if len(selected) != 1 || len(selected[0]) > 10 {
		return 0, fmt.Errorf("%w: library %s is invalid", ErrInvalidBrowse, name)
	}
	value, err := strconv.Atoi(selected[0])
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%w: library %s is invalid", ErrInvalidBrowse, name)
	}
	return value, nil
}

func pageURL(values url.Values, offset, limit int, visible bool) string {
	if !visible {
		return ""
	}
	query := cloneValues(values)
	query.Set("offset", strconv.Itoa(offset))
	query.Set("limit", strconv.Itoa(limit))
	return "/?" + query.Encode()
}

func cloneValues(values url.Values) url.Values {
	result := make(url.Values, len(values))
	for key, selected := range values {
		result[key] = append([]string(nil), selected...)
	}
	return result
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}
