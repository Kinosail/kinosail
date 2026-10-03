package servertest

import (
	"regexp"
	"testing"
)

// SupportedLocale is one advertised locale and its text direction.
type SupportedLocale struct{ Tag, Direction string }

var localePlaceholder = regexp.MustCompile(`{{[^{}]+}}|\{[A-Za-z][A-Za-z0-9_]*}`)

// LocaleCatalogsAreComplete verifies catalog cardinality and placeholder preservation.
func LocaleCatalogsAreComplete(t *testing.T, locales []SupportedLocale, read func(*testing.T, string) map[string]string) {
	t.Helper()
	english := read(t, "en")
	for _, supported := range locales {
		catalog := read(t, supported.Tag)
		if len(catalog) != len(english) {
			t.Fatalf("%s catalog has %d messages; want %d", supported.Tag, len(catalog), len(english))
		}
		for id, source := range english {
			translated, ok := catalog[id]
			if !ok || translated == "" {
				t.Fatalf("%s catalog is missing %q", supported.Tag, id)
			}
			got, want := localePlaceholder.FindAllString(translated, -1), localePlaceholder.FindAllString(source, -1)
			if !sameStrings(got, want) {
				t.Fatalf("%s placeholders for %q = %q; want %q", supported.Tag, id, got, want)
			}
		}
	}
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
