package localization

import "testing"

func TestLocalizedPhrasePreservesBoundaries(t *testing.T) {
	for _, tc := range []struct{ source, phrase, translation, want string }{
		{"Play playlist Play", "Play", "Lire", "Lire playlist Lire"},
		{"Playback Play replayPlay Play2", "Play", "Lire", "Playback Lire replayPlay Play2"},
		{"<p>Nothing matches here</p>", "Play", "Lire", "<p>Nothing matches here</p>"},
		{"Play", "", "Lire", "Play"},
		{"Play Play", "Play", "Play again", "Play again Play again"},
		{"(Play) Play!", "Play", "", "() !"},
	} {
		if got := replaceLocalizedPhrase(tc.source, tc.phrase, tc.translation); got != tc.want {
			t.Errorf("replace %q in %q = %q, want %q", tc.phrase, tc.source, got, tc.want)
		}
	}
}

func TestAbsentLocalizedPhraseDoesNotCopySource(t *testing.T) {
	source := "An unchanged template text node"
	allocations := testing.AllocsPerRun(100, func() {
		if got := replaceLocalizedPhrase(source, "unmatched catalog phrase", "translation"); got != source {
			t.Fatal("absent phrase changed source")
		}
	})
	if allocations != 0 {
		t.Fatalf("absent phrase allocated %g times", allocations)
	}
}
