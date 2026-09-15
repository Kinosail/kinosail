package catalog

import (
	"sort"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/unicode/norm"
)

// Filter returns items that match a normalized Library search.
func Filter(items []library.Item, query string) []library.Item {
	if query == "" {
		return items
	}
	matched := make([]library.Item, 0)
	for _, item := range items {
		if Matches(item, query) {
			matched = append(matched, item)
		}
	}
	return matched
}

// Matches reports whether searchable item metadata contains the query.
func Matches(item library.Item, query string) bool {
	var credits strings.Builder
	for _, person := range item.Cast {
		credits.WriteByte(' ')
		credits.WriteString(person.Name)
		credits.WriteByte(' ')
		credits.WriteString(person.Role)
	}
	for _, person := range item.ShowCast {
		credits.WriteByte(' ')
		credits.WriteString(person.Name)
		credits.WriteByte(' ')
		credits.WriteString(person.Role)
	}
	normalized := searchText(query)
	return normalized != "" && strings.Contains(searchText(item.Title+" "+item.Show+" "+item.Year+" "+item.Plot+" "+item.Genres+" "+item.Director+" "+item.Studio+" "+item.Artist+" "+item.Album+credits.String()), normalized)
}

// Sort orders Library items by the supported smart-list order.
func Sort(items []library.Item, order string) []library.Item {
	switch order {
	case "added":
		sort.Slice(items, func(left, right int) bool {
			return items[left].Added.After(items[right].Added) || items[left].Added.Equal(items[right].Added) && items[left].ID < items[right].ID
		})
	case "year":
		sort.Slice(items, func(left, right int) bool {
			return items[left].Year > items[right].Year || items[left].Year == items[right].Year && items[left].ID < items[right].ID
		})
	default:
		sort.Slice(items, func(left, right int) bool {
			leftTitle, rightTitle := sortTitle(items[left]), sortTitle(items[right])
			return leftTitle < rightTitle || leftTitle == rightTitle && items[left].ID < items[right].ID
		})
	}
	return items
}

func searchRank(item library.Item, query string) int {
	title, query := searchText(item.Title), searchText(query)
	switch {
	case title == query:
		return 0
	case strings.HasPrefix(title, query):
		return 1
	case strings.HasPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(title, "the "), "an "), "a "), query):
		return 2
	case strings.Contains(" "+title, " "+query):
		return 3
	case strings.Contains(title, query):
		return 4
	default:
		return 5
	}
}

func searchText(value string) string {
	var result strings.Builder
	space := true
	for _, character := range norm.NFKD.String(strings.ToLower(value)) {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			space = false
		} else if !space {
			result.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(result.String())
}

// Media selects items of one media kind, preserving their input order.
func Media(items []library.Item, kind string) []library.Item {
	matched := make([]library.Item, 0)
	for _, item := range items {
		if item.Kind == kind {
			matched = append(matched, item)
		}
	}
	return matched
}
