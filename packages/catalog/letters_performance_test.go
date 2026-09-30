package catalog_test

import (
	"net/url"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestBrowseLetterJumpsPreserveLocaleAndUnicode(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ title, locale, letter string }{
		{"alpha", "en", "A"},
		{"Zebra", "en", "Z"},
		{"'iris", "tr", "I"},
		{"iris", "az", "I"},
		{"iris", "lt", "I"},
		{"éclair", "fr", "E"},
		{"e\u0301clair", "fr", "E"},
		{"İstanbul", "tr", "I"},
		{"ışık", "tr", "I"},
		{"ωμέγα", "el", "Ω"},
		{"東京", "ja", "東"},
		{"ßound", "de", "SS"},
		{" 7 wonders", "en", "#"},
		{"!!!", "en", "#"},
	} {
		t.Run(test.locale+"/"+test.title, func(t *testing.T) {
			browse, err := catalog.ParseBrowse(url.Values{"letter": {test.letter}}, test.locale)
			if err != nil {
				t.Fatal(err)
			}
			item := library.Item{ID: "item", Title: test.title}
			result, err := browse.Apply(t.Context(), []catalog.Candidate{{Item: &item}})
			if err != nil || len(result.Items) != 1 || len(result.Letters) != 1 || result.Letters[0].Label != test.letter || !result.Letters[0].Current {
				t.Fatalf("browse %q (%s) = %#v, %v", test.title, test.locale, result, err)
			}
		})
	}
}
