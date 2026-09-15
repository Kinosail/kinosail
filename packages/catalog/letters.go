package catalog

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func parseLetter(values url.Values, locale string) (string, error) {
	present := values.Has("letter")
	value, err := singleValue(values, "letter", 16)
	value = strings.TrimSpace(value)
	if err != nil || present && value == "" || value != "" && value != "#" && (utf8.RuneCountInString(value) > 4 || !lettersOnly(value)) {
		return "", fmt.Errorf("%w: library letter is invalid", ErrInvalidBrowse)
	}
	if value == "" {
		return "", nil
	}
	return normalizeLetter(value, locale), nil
}

func browseLetters(values url.Values, items []*library.Item, current, locale string) []Letter {
	letters := make([]Letter, 0, 26)
	for offset, item := range items {
		label := titleLetter(sortTitle(*item), locale)
		if len(letters) == 0 || letters[len(letters)-1].Label != label {
			query := cloneValues(values)
			query.Del("offset")
			selected := label == current
			if selected {
				query.Del("letter")
			} else {
				query.Set("letter", label)
			}
			letters = append(letters, Letter{Label: label, Offset: offset, Href: "/?" + query.Encode(), Current: selected})
		}
		letters[len(letters)-1].Count++
	}
	return letters
}

func showReferences(items []*library.Item) []*library.Item {
	shows := make(map[string]*library.Item, len(items))
	ordered := make([]*library.Item, 0, len(items))
	for _, item := range items {
		if item.Kind != "video" || item.Show == "" {
			continue
		}
		key := strings.ToLower(item.Show)
		show, found := shows[key]
		if !found {
			show = newShowReference(item)
			shows[key] = show
			ordered = append(ordered, show)
			continue
		}
		mergeShowReference(show, item)
	}
	return ordered
}

func newShowReference(item *library.Item) *library.Item {
	copy := *item
	copy.Title = copy.Show
	if copy.ShowTitle != "" {
		copy.Title = copy.ShowTitle
	}
	copy.SortTitle = copy.Title
	return &copy
}

func mergeShowReference(show, item *library.Item) { //nolint:cyclop // Each condition fills one independent optional presentation field.
	if item.ShowTitle != "" {
		show.ShowTitle, show.Title, show.SortTitle = item.ShowTitle, item.ShowTitle, item.ShowTitle
	}
	if show.ShowYear == "" {
		show.ShowYear = item.ShowYear
	}
	if show.ShowPlot == "" {
		show.ShowPlot = item.ShowPlot
	}
	if show.ShowGenres == "" {
		show.ShowGenres = item.ShowGenres
	}
	if show.ShowStudio == "" {
		show.ShowStudio = item.ShowStudio
	}
	if show.ShowArtwork == "" {
		show.ShowArtwork = item.ShowArtwork
	}
	if show.ShowBackdrop == "" {
		show.ShowBackdrop = item.ShowBackdrop
	}
	if show.ShowLogo == "" {
		show.ShowLogo = item.ShowLogo
	}
	if len(show.ShowCast) == 0 {
		show.ShowCast = item.ShowCast
	}
}

func sortTitle(item library.Item) string {
	if item.SortTitle != "" {
		return item.SortTitle
	}
	return item.Title
}

func sortKey(item library.Item) string {
	title := sortTitle(item)
	if len(title) > 0 && (title[0] >= '0' && title[0] <= '9' || title[0] >= 'A' && title[0] <= 'Z' || title[0] >= 'a' && title[0] <= 'z') {
		return title
	}
	return strings.TrimLeftFunc(title, func(value rune) bool { return !unicode.IsLetter(value) && !unicode.IsDigit(value) })
}

func titleLetter(title, locale string) string {
	for _, value := range strings.TrimSpace(title) {
		if unicode.IsLetter(value) {
			return normalizeLetter(string(value), locale)
		}
		if unicode.IsDigit(value) {
			return "#"
		}
	}
	return "#"
}

func lettersOnly(value string) bool {
	found := false
	for _, current := range norm.NFD.String(value) {
		if unicode.IsLetter(current) {
			found = true
		} else if !unicode.Is(unicode.Mn, current) {
			return false
		}
	}
	return found
}

func normalizeLetter(value, locale string) string {
	value = cases.Upper(language.Make(locale)).String(value)
	var normalized strings.Builder
	for _, current := range norm.NFD.String(value) {
		if !unicode.Is(unicode.Mn, current) {
			normalized.WriteRune(current)
		}
	}
	return normalized.String()
}
