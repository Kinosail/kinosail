package catalog

import (
	"net/url"
	"strings"
	"testing"
)

func TestCurationValidation(t *testing.T) {
	t.Parallel()

	for name, form := range map[string]url.Values{
		"valid":         {"included": {"false"}, "_csrf": {"token"}, "q": {"Arrival"}},
		"without query": {"included": {"true"}},
	} {
		if !ValidCurationItemForm(form) {
			t.Errorf("%s form rejected: %#v", name, form)
		}
	}
	for name, form := range map[string]url.Values{
		"unknown field":    {"included": {"false"}, "extra": {"value"}},
		"invalid included": {"included": {"maybe"}},
		"duplicate csrf":   {"included": {"false"}, "_csrf": {"one", "two"}},
		"oversized csrf":   {"included": {"false"}, "_csrf": {strings.Repeat("x", 129)}},
		"duplicate query":  {"included": {"false"}, "q": {"one", "two"}},
		"oversized query":  {"included": {"false"}, "q": {strings.Repeat("x", 201)}},
		"invalid query":    {"included": {"false"}, "q": {string([]byte{0xff})}},
	} {
		if ValidCurationItemForm(form) {
			t.Errorf("%s form unexpectedly accepted: %#v", name, form)
		}
	}

	for name, value := range map[string]bool{
		"valid":        ValidCollectionPathName("A Collection"),
		"empty":        ValidCollectionPathName(""),
		"slash":        ValidCollectionPathName("A/B"),
		"oversized":    ValidCollectionPathName(strings.Repeat("x", 201)),
		"invalid utf8": ValidCollectionPathName(string([]byte{0xff})),
	} {
		want := name == "valid"
		if value != want {
			t.Errorf("collection name %s = %t, want %t", name, value, want)
		}
	}

	for name, value := range map[string]bool{
		"valid":      ValidCurationItemID("0123456789abcdef"),
		"short":      ValidCurationItemID("0123456789abcde"),
		"non-hex":    ValidCurationItemID("0123456789abcdeg"),
		"odd length": ValidCurationItemID("0123456789abcdef0"),
	} {
		want := name == "valid"
		if value != want {
			t.Errorf("item ID %s = %t, want %t", name, value, want)
		}
	}
}
