package server

import (
	"strings"
	"testing"
	"time"
)

func TestCleanSubtitleNormalizesAndRemovesOnlyClearJunk(t *testing.T) {
	t.Parallel()
	input := "\ufeffWEBVTT\r\n\r\n00:00:05.000 --> 00:00:06.000 align:start\r\n<font color=red>Downloaded from www.example.test</font>\r\n\r\n00:00:01.000 --> 00:00:03.000\r\n{\\an8}<i>Cafe\u0301   hello</i>\r\n\r\n00:00:01.000 --> 00:00:03.000\r\n{\\an8}<i>Cafe\u0301   hello</i>\r\n"
	cleaned, err := convertSubtitle([]byte(input), "en", subtitleConversionOptions{RemoveCredits: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(cleaned.Data)
	if len(cleaned.Cues) != 1 || cleaned.Duplicates != 1 || !strings.Contains(text, "Café hello") || strings.Contains(text, "Downloaded") || !strings.Contains(text, "00:00:01,000 --> 00:00:03,000") {
		t.Fatalf("cleaned = %#v\n%s", cleaned, text)
	}
}

func TestCleanSubtitlePreservesRepeatedAdjacentCuesAndBalancesFormatting(t *testing.T) {
	t.Parallel()
	input := "1\n00:00:01,000 --> 00:00:02,000\n<i>Hello\n\n2\n00:00:02,050 --> 00:00:03,000\n<i>Hello\n\n3\n00:00:04,000 --> 00:00:05,000\n<b>Stay</b> <u>here</u>\n"
	cleaned, err := cleanSubtitle([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	text := string(cleaned.Data)
	if len(cleaned.Cues) != 3 || cleaned.Duplicates != 0 || !strings.Contains(text, "00:00:01,000 --> 00:00:02,000\nHello") || !strings.Contains(text, "<b>Stay</b> <u>here</u>") || strings.Contains(text, "<i>") {
		t.Fatalf("cleaned = %#v\n%s", cleaned, text)
	}
}

func TestCleanSubtitlePreservesMeaningfulAccessibilityText(t *testing.T) {
	t.Parallel()
	input := "1\n00:00:01,000 --> 00:00:02,000\n[door closes]\n\n2\n00:00:03,000 --> 00:00:04,000\n- ALEX: Wait.\n\n3\n00:00:05,000 --> 00:00:06,000\n♪ We are alive ♪\n\n4\n00:00:08,000 --> 00:00:09,000\nWait.\n\n5\n00:00:10,000 --> 00:00:11,000\nWait.\n"
	cleaned, err := cleanSubtitle([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	text := string(cleaned.Data)
	for _, meaningful := range []string{"[door closes]", "- ALEX: Wait.", "♪ We are alive ♪"} {
		if !strings.Contains(text, meaningful) {
			t.Fatalf("meaningful text %q was removed:\n%s", meaningful, text)
		}
	}
	if len(cleaned.Cues) != 5 || cleaned.Duplicates != 0 {
		t.Fatalf("intentional repetition changed: %#v\n%s", cleaned, text)
	}
}

func TestCleanSubtitleRemovesCreditsFromChronologicalEdges(t *testing.T) {
	t.Parallel()
	input := "1\n00:00:03,000 --> 00:00:04,000\nThree\n\n2\n00:00:04,000 --> 00:00:05,000\nFour\n\n3\n00:00:05,000 --> 00:00:06,000\nFive\n\n4\n00:00:01,000 --> 00:00:02,000\nDownloaded from www.example.test\n\n5\n00:00:06,000 --> 00:00:07,000\nSix\n\n6\n00:00:07,000 --> 00:00:08,000\nSeven\n\n7\n00:00:08,000 --> 00:00:09,000\nEight\n"
	cleaned, err := convertSubtitle([]byte(input), "en", subtitleConversionOptions{RemoveCredits: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cleaned.Data), "Downloaded") || len(cleaned.Cues) != 6 {
		t.Fatalf("chronological edge credit was retained: %#v\n%s", cleaned, cleaned.Data)
	}
}

func TestCleanSubtitleRejectsMalformedInput(t *testing.T) { //nolint:gosec // Invalid subtitle fixtures intentionally contain URL-like text.
	t.Parallel()
	for name, input := range map[string]string{ //nolint:gosec // Fixture contains invalid subtitle text with URL-like noise.
		"binary":     "1\n00:00:01,000 --> 00:00:02,000\nhello\x00",
		"reverse":    "1\n00:00:02,000 --> 00:00:01,000\nhello",
		"bad minute": "1\n00:61:01,000 --> 00:61:02,000\nhello",
		"empty":      "WEBVTT\n\nNOTE no cues",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := cleanSubtitle([]byte(input)); err == nil {
				t.Fatalf("accepted %q", input)
			}
		})
	}
}

func TestSubtitleDocumentRejectsNegativeAlignedCue(t *testing.T) {
	t.Parallel()
	if _, err := subtitleDocument([]subtitleCue{{-time.Second, time.Second, "hello"}}, 0); err == nil {
		t.Fatal("negative cue was accepted")
	}
}
