package catalog

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/unicode/norm"
)

// Filter returns items that match a normalized Library search.
func Filter(items []library.Item, query string) []library.Item {
	if query == "" {
		return items
	}
	normalized := searchText(query)
	matched := make([]library.Item, 0)
	for _, item := range items {
		if matchesNormalized(item, normalized) {
			matched = append(matched, item)
		}
	}
	return matched
}

// Matches reports whether searchable item metadata contains the query.
func Matches(item library.Item, query string) bool {
	return matchesNormalized(item, searchText(query))
}

func matchesNormalized(item library.Item, normalized string) bool {
	if normalized == "" {
		return false
	}
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
	return strings.Contains(searchText(item.Title+" "+item.Show+" "+item.Year+" "+item.Plot+" "+item.Genres+" "+item.Director+" "+item.Studio+" "+item.Artist+" "+item.Album+credits.String()), normalized)
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
	return searchRankNormalized(item, searchText(query))
}

func searchRankNormalized(item library.Item, query string) int {
	title := searchText(item.Title)
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
	if isASCII(value) {
		return asciiSearchText(value)
	}
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

func asciiSearchText(value string) string {
	var result strings.Builder
	result.Grow(len(value))
	space := true
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			result.WriteByte(character)
			space = false
		} else if !space {
			result.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(result.String())
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= utf8.RuneSelf {
			return false
		}
	}
	return true
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
