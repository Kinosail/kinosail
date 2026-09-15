package subtitlelanguage

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogDeduplicatesOverlappingIdentitySources(t *testing.T) {
	original := Catalog()
	oldVariants, oldChoices, oldKnown := variantTags, choices, known
	t.Cleanup(func() { variantTags, choices, known = oldVariants, oldChoices, oldKnown })
	variantTags = append(append([]string(nil), variantTags...), "en", "pt-BR")
	choices = nil
	initCatalog()
	if got := Catalog(); !reflect.DeepEqual(got, original) {
		t.Fatal("duplicate source identities changed the catalog")
	}
}

func TestLanguageTagSyntaxRejectsMalformedInput(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("x", 65), "e1", "en-@@"} {
		if validTagSyntax(value) {
			t.Errorf("invalid tag syntax accepted: %q", value)
		}
	}
}

func TestLanguageNormalizationRejectsUnregisteredAndMalformedInput(t *testing.T) {
	if value, ok := NormalizeLocal("tlh-Latn"); ok || value != "" {
		t.Fatalf("unselectable registered base accepted: %q %v", value, ok)
	}
	for _, value := range []string{"", " en", strings.Repeat("x", 65)} {
		if canonical, ok := NormalizeProvider(value, SubDL); ok || canonical != "" {
			t.Errorf("invalid provider value accepted: %q", value)
		}
	}
	if value, ok := NormalizeProvider("en", Provider("unknown")); ok || value != "" {
		t.Fatalf("unknown provider normalized: %q %v", value, ok)
	}
	if value, ok := Code("en", Provider("unknown")); ok || value != "" {
		t.Fatalf("unknown provider code: %q %v", value, ok)
	}
}
