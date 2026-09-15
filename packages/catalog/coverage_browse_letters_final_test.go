package catalog

import (
	"errors"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCoverageBrowseTitleOrderMatchesUniqueLetterBuckets(t *testing.T) {
	items := []*library.Item{
		{ID: "quoted", Title: "'Cruella"},
		{ID: "beta-b", Title: "Beta"},
		{ID: "number", Title: "10 Things"},
		{ID: "plain", Title: "Cruella"},
		{ID: "beta-a", Title: "Beta"},
	}
	sortReferences(items, "title", "", "en")
	if got, want := itemIDs(items), []string{"number", "beta-a", "beta-b", "plain", "quoted"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("title order = %v, want %v", got, want)
	}

	values := url.Values{"letter": {"B"}, "offset": {"999"}, "lang": {"en"}}
	before := cloneValues(values)
	want := []Letter{
		{Label: "#", Count: 1, Offset: 0, Href: "/?lang=en&letter=%23"},
		{Label: "B", Count: 2, Offset: 1, Href: "/?lang=en", Current: true},
		{Label: "C", Count: 2, Offset: 3, Href: "/?lang=en&letter=C"},
	}
	if got := browseLetters(values, items, "B", "en"); !reflect.DeepEqual(got, want) {
		t.Fatalf("letter buckets = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(values, before) {
		t.Fatalf("letter projection mutated values: got %v, want %v", values, before)
	}
	if got := browseLetters(values, nil, "", "en"); len(got) != 0 {
		t.Fatalf("empty letter buckets = %#v", got)
	}
}

func TestCoverageHistorySortAndBrowseApplyKeepInputsStable(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	items := []library.Item{
		{ID: "old", Title: "Old"},
		{ID: "b", Title: "Beta"},
		{ID: "a", Title: "Alpha"},
	}
	history := []Candidate{
		{Item: &items[0], Updated: now.Add(-time.Hour)},
		{Item: &items[1], Updated: now},
		{Item: &items[2], Updated: now},
	}
	sortHistory(history)
	if got, want := candidateIDs(history), []string{"a", "b", "old"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("history order = %v, want %v", got, want)
	}

	values := url.Values{"limit": {"2"}}
	browse, err := ParseBrowse(values, "en")
	if err != nil {
		t.Fatal(err)
	}
	values.Set("limit", "1")
	candidates := []Candidate{{Item: &items[1]}, {Item: nil}, {Item: &items[2]}}
	beforeCandidates := append([]Candidate(nil), candidates...)
	beforeItems := append([]library.Item(nil), items...)
	result, err := browse.Apply(candidates)
	if err != nil || result.Limit != 2 || result.Total != 2 || !reflect.DeepEqual(itemValueIDs(result.Items), []string{"a", "b"}) {
		t.Fatalf("browse result = %#v, error=%v", result, err)
	}
	if !reflect.DeepEqual(candidates, beforeCandidates) || !reflect.DeepEqual(items, beforeItems) {
		t.Fatal("browse application mutated candidate or item input")
	}
	result.Items[0].Title = "Changed"
	if items[2].Title != "Alpha" {
		t.Fatal("browse page exposed item value storage")
	}
}

func TestCoverageSearchUnicodeRankingAndFilterIsolation(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{title: "Café", want: 0},
		{title: "Cafeteria", want: 1},
		{title: "An Café", want: 2},
		{title: "Night—Café", want: 3},
		{title: "Decaféiné", want: 4},
		{title: "Tea", want: 5},
	}
	assertSearchRanks(t, tests)
	assertSearchNormalization(t)
	assertFilterIsolation(t)
}

func assertSearchRanks(t *testing.T, tests []struct {
	title string
	want  int
},
) {
	t.Helper()
	references := make([]*library.Item, len(tests))
	for index, test := range tests {
		if got := searchRank(library.Item{Title: test.title}, "cafe"); got != test.want {
			t.Fatalf("rank %q = %d, want %d", test.title, got, test.want)
		}
		references[len(tests)-1-index] = &library.Item{ID: test.title, Title: test.title}
	}
	sortReferences(references, "title", "cafe", "fr")
	for index, item := range references {
		if item.Title != tests[index].title {
			t.Fatalf("ranked order %d = %q, want %q", index, item.Title, tests[index].title)
		}
	}
}

func assertSearchNormalization(t *testing.T) {
	t.Helper()
	for input, want := range map[string]string{
		"":                         "",
		"!!!":                      "",
		" Éléphant—CAFÉ_42 ":       "elephant cafe 42",
		"A\u0301 la carte\t2026!!": "a la carte 2026",
	} {
		if got := searchText(input); got != want {
			t.Fatalf("searchText(%q) = %q, want %q", input, got, want)
		}
	}
	if Matches(library.Item{Title: "Café"}, "") || Matches(library.Item{Title: "Café"}, "!!!") {
		t.Fatal("empty normalized search matched an item")
	}
}

func assertFilterIsolation(t *testing.T) {
	t.Helper()
	source := []library.Item{{ID: "one", Title: "Café"}, {ID: "two", Title: "Tea"}}
	before := append([]library.Item(nil), source...)
	matched := Filter(source, "CAFE")
	if len(matched) != 1 || matched[0].ID != "one" {
		t.Fatalf("unicode filter = %#v", matched)
	}
	matched[0].Title = "Changed"
	if !reflect.DeepEqual(source, before) {
		t.Fatal("filtered result exposed source item values")
	}
}

func TestCoverageLetterParsingUnicodeEmptyAndDuplicateBoundaries(t *testing.T) {
	valid := []struct {
		values url.Values
		locale string
		want   string
	}{
		{values: nil, locale: "en", want: ""},
		{values: url.Values{"letter": {"#"}}, locale: "en", want: "#"},
		{values: url.Values{"letter": {"  é  "}}, locale: "fr", want: "E"},
		{values: url.Values{"letter": {"e\u0301"}}, locale: "fr", want: "E"},
		{values: url.Values{"letter": {"ω"}}, locale: "el", want: "Ω"},
	}
	for _, test := range valid {
		if got, err := parseLetter(test.values, test.locale); err != nil || got != test.want {
			t.Fatalf("parseLetter(%v, %q) = %q, %v; want %q", test.values, test.locale, got, err, test.want)
		}
	}
	invalidUTF8 := string([]byte{0xff})
	for _, values := range []url.Values{
		{"letter": {""}},
		{"letter": {"A", "A"}},
		{"letter": {"A1"}},
		{"letter": {"ABCDE"}},
		{"letter": {"?"}},
		{"letter": {invalidUTF8}},
	} {
		if _, err := parseLetter(values, "en"); !errors.Is(err, ErrInvalidBrowse) {
			t.Fatalf("invalid letter %v returned %v", values, err)
		}
	}
	if lettersOnly("\u0301") || titleLetter(" '\u0301 7", "en") != "#" || titleLetter(" 'éclair", "fr") != "E" {
		t.Fatal("letter helpers did not preserve Unicode and empty boundaries")
	}
}

func itemIDs(items []*library.Item) []string {
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.ID
	}
	return result
}

func itemValueIDs(items []library.Item) []string {
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.ID
	}
	return result
}

func candidateIDs(candidates []Candidate) []string {
	result := make([]string, len(candidates))
	for index, candidate := range candidates {
		result[index] = candidate.Item.ID
	}
	return result
}
