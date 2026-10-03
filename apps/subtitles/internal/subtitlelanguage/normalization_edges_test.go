package subtitlelanguage

import (
	"strings"
	"testing"
)

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
