package webassets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestApplicationStylesPreserveFrozenBytes(t *testing.T) {
	t.Parallel()

	for name, fixture := range map[string]struct {
		content []byte
		length  int
		digest  string
	}{
		"player":    {PlayerCSS, 150_338, "10213b1793951c2cd1a138c9884f471bf3efef8d0075783d67d9becbd43a4cdd"},
		"subtitles": {SubtitlesCSS, 160_572, "28f8730df6b0b3270f0f154560b7c25acddc0999b182808d58fb780d35a399e7"},
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
