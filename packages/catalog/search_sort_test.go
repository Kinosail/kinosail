package catalog

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSearchSortingPreservesRelevanceAndTieBreaks(t *testing.T) {
	items := []library.Item{
		{ID: "d", Title: "Café nights", Year: "2027", Added: time.Unix(3, 0)},
		{ID: "b", Title: "Café", Year: "2025", Added: time.Unix(1, 0)},
		{ID: "c", Title: "Café", Year: "2026", Added: time.Unix(2, 0)},
		{ID: "a", Title: "Café", Year: "2025", Added: time.Unix(1, 0)},
	}
	candidates := make([]Candidate, len(items))
	for index := range items {
		candidates[index] = Candidate{Item: &items[index]}
	}
	for order, want := range map[string][]string{
		"title": {"a", "b", "c", "d"},
		"added": {"c", "a", "b", "d"},
		"year":  {"c", "a", "b", "d"},
	} {
		browse, err := ParseBrowse(url.Values{"q": {"CAFÉ"}, "sort": {order}}, "fr")
		if err != nil {
			t.Fatal(err)
		}
		result, err := browse.Apply(candidates)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(result.Items))
		for index, item := range result.Items {
			ids[index] = item.ID
		}
		if !reflect.DeepEqual(ids, want) || items[0].ID != "d" {
			t.Fatalf("%s sorting = %v; want %v", order, ids, want)
		}
	}
}

func TestSearchTextNormalizesASCIIAndUnicodeMetadata(t *testing.T) {
	for input, want := range map[string]string{
		" The.DARK\tKnight! 2026 ": "the dark knight 2026",
		"_--_":                     "",
		"A+B/C":                    "a b c",
		"Été & Summer":             "ete summer",
	} {
		if got := searchText(input); got != want {
			t.Errorf("searchText(%q) = %q, want %q", input, got, want)
		}
	}
}
