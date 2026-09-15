package servertest

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// UIIconSetIsCompleteAndLocal runs the corresponding app regression contract.
func UIIconSetIsCompleteAndLocal(t *testing.T, uiIcon func(string) template.HTML) {
	t.Parallel()

	for _, name := range []string{"audiobook", "back", "book", "cast", "check", "collection", "music", "photo", "play", "playlist", "plus", "shows", "star"} {
		markup := string(uiIcon(name))
		if !strings.Contains(markup, `class=i-`+name) || !strings.Contains(markup, `aria-hidden=true`) || strings.Contains(markup, "http") {
			t.Fatalf("icon %q = %q", name, markup)
		}
	}
	if uiIcon("unknown") != "" {
		t.Fatal("unknown icons must render nothing")
	}
}

// ProductionUITemplatesAvoidMixedGlyphIcons runs the corresponding app regression contract.
func ProductionUITemplatesAvoidMixedGlyphIcons(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(string(data), "▶★＋↻♫◉▤◆≡✓◇▥←") {
			t.Errorf("%s still contains a mixed glyph icon", entry.Name())
		}
	}
}
