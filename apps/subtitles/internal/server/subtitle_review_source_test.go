package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleReviewRejectsInvalidLanguageBeforeReading(t *testing.T) {
	root := t.TempDir()
	item := library.Item{Path: filepath.Join(root, "film.mp4")}
	for _, language := range []string{"", "../en", "en/../../secret", "/en", "en\\secret", "en\x00", strings.Repeat("x", 300), "unknown-language"} {
		path, data, err := subtitleReviewSource(item, language)
		if err == nil || path != "" || data != nil {
			t.Fatalf("invalid language %q reached source: %q %v", language, path, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected language changed filesystem")
	}
}

func TestSubtitleReviewCanonicalizesLanguageBeforeReading(t *testing.T) {
	item := library.Item{Path: filepath.Join(t.TempDir(), "film.mp4")}
	path := subtitleSidecarPath(item, "en")
	content := []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	got, data, err := subtitleReviewSource(item, "EN")
	if err != nil || got != path || string(data) != string(content) {
		t.Fatalf("canonical source = %q, %v", got, err)
	}
}
