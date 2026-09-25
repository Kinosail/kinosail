package webassets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestApplicationStylesPreserveFrozenBytes(t *testing.T) {
	t.Parallel()

	for name, fixture := range map[string]struct {
		content []byte
		length  int
		digest  string
	}{
		"player":    {PlayerCSS, 153_324, "9a796aaa81b49480e576f96df1aefb3392778fe51709f0c2b10fc92cad8b08c4"},
		"subtitles": {SubtitlesCSS, 163_523, "e3b4307c93f723435c18ef5bcaea3eb6639df1a8b8c8ffefeae6cb74c655d7eb"},
	} {
		if digest := fmt.Sprintf("%x", sha256.Sum256(fixture.content)); len(fixture.content) != fixture.length || digest != fixture.digest {
			t.Fatalf("%s stylesheet = %d bytes, %s", name, len(fixture.content), digest)
		}
	}
}

func TestForegroundArtworkPreservesItsSourceRatio(t *testing.T) {
	t.Parallel()

	if !bytes.Contains(LastLightCSS, []byte("body :is(img.poster,.poster img,.card img,.hero-poster){height:auto;object-fit:contain}")) ||
		!bytes.Contains(LastLightCSS, []byte("body :is(.curation-poster>img,.collection-poster>img){object-fit:contain}")) {
		t.Fatal("shared light stylesheet does not preserve foreground artwork")
	}
}

func TestMediaFocusRingStaysInsideArtwork(t *testing.T) {
	t.Parallel()

	css := string(LastLightCSS)
	for _, fragment := range []string{
		"body :is(.card,.curation-card,.collection-card):has(.poster,.curation-poster,.collection-poster):focus-visible{outline:0}",
		"outline:3px solid var(--signal);outline-offset:-3px",
	} {
		if !bytes.Contains(LastLightCSS, []byte(fragment)) {
			t.Fatalf("shared stylesheet is missing media focus treatment %q", fragment)
		}
	}
	if strings.Contains(css, "box-shadow:inset 0 0 0 3px var(--signal)") {
		t.Fatal("media highlight still relies on an inset shadow that image posters can hide")
	}
	if strings.Contains(css, ".card:focus-visible .poster){transform:none;outline:2px solid var(--text)") {
		t.Fatal("media focus still uses the clipped text-color outline")
	}
}

func TestApplyStylePatchSupportsUnifiedHunks(t *testing.T) {
	t.Parallel()

	source := []byte("one\ntwo\nthree\nfive\n")
	patch := []byte("--- player.css\n+++ subtitles.css\n@@ -0,0 +1 @@\n+zero\n@@ -1,2 +2,2 @@\n one\n-two\n+TWO\n@@ -3,0 +5 @@\n+four\n")
	result, err := applyStylePatch(source, patch)
	if err != nil || !bytes.Equal(result, []byte("zero\none\nTWO\nthree\nfour\nfive\n")) {
		t.Fatalf("patched stylesheet = %q, %v", result, err)
	}
}

func TestStylePatchRejectsInvalidInput(t *testing.T) { //nolint:cyclop // One table covers each trusted patch format invariant.
	t.Parallel()

	source := []byte("one\ntwo\n")
	for name, patch := range map[string]string{
		"missing headers":     "",
		"invalid target":      "--- a\ninvalid\n",
		"invalid hunk":        "--- a\n+++ b\ninvalid\n",
		"invalid old range":   "--- a\n+++ b\n@@ -x +1 @@\n+one\n",
		"invalid new range":   "--- a\n+++ b\n@@ -1 +x @@\n-one\n",
		"hunk after source":   "--- a\n+++ b\n@@ -4,0 +4 @@\n+four\n",
		"overlapping hunks":   "--- a\n+++ b\n@@ -1 +1 @@\n-one\n+ONE\n@@ -1 +1 @@\n-two\n+TWO\n",
		"removed mismatch":    "--- a\n+++ b\n@@ -1 +1 @@\n-other\n+ONE\n",
		"removed after end":   "--- a\n+++ b\n@@ -3 +3 @@\n-three\n+THREE\n",
		"context mismatch":    "--- a\n+++ b\n@@ -1 +1 @@\n other\n",
		"context after end":   "--- a\n+++ b\n@@ -3 +3 @@\n three\n",
		"invalid marker":      "--- a\n+++ b\n@@ -1 +1 @@\n?one\n",
		"wrong removed count": "--- a\n+++ b\n@@ -1,2 +1 @@\n-one\n+ONE\n",
		"wrong added count":   "--- a\n+++ b\n@@ -1 +1,2 @@\n-one\n+ONE\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := applyStylePatch(source, []byte(patch)); err == nil {
				t.Fatal("invalid stylesheet patch was accepted")
			}
		})
	}
}

func TestStylePatchParsersRejectInvalidRanges(t *testing.T) { //nolint:cyclop // One parser matrix remains below the repository complexity ceiling.
	t.Parallel()

	if _, _, err := (&stylePatchState{}).applyLine(nil); err == nil {
		t.Fatal("empty patch line was accepted")
	}
	for _, value := range []string{"x", "-1", "1,x", "1,-1"} {
		if _, _, err := parseStyleRange(value); err == nil {
			t.Fatalf("range %q was accepted", value)
		}
	}
	for _, hunk := range []string{"@@ 1 +1 @@", "@@ -1 1 @@"} {
		if _, _, _, err := parseStyleHunk([]byte(hunk)); err == nil {
			t.Fatalf("hunk %q was accepted", hunk)
		}
	}
	start, count, err := parseStyleRange("2")
	if err != nil || start != 2 || count != 1 {
		t.Fatalf("default range = %d, %d, %v", start, count, err)
	}
	if lines := splitStyleLines(nil); lines != nil {
		t.Fatalf("empty lines = %#v", lines)
	}
	if lines := splitStyleLines([]byte("one")); len(lines) != 1 || !bytes.Equal(lines[0], []byte("one")) {
		t.Fatalf("unterminated lines = %#v", lines)
	}
}

func TestMustApplyStylePatchPanicsOnInvalidPatch(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("invalid embedded stylesheet patch did not panic")
		}
	}()
	mustApplyStylePatch(nil, nil)
}
