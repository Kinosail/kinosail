package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestReleaseCompatibilitySeparatesTitleFromEdition(t *testing.T) {
	for _, pair := range [][2]string{
		{"Arrival.2016", "Arrival.2016"},
		{"Arrival.2016.1080p.BluRay-GROUP", "Arrival.2016.1080p.WEB-DL-GROUP"},
		{"Arrival.2016.Extended.BluRay-GROUP", "Arrival.2016.BluRay-GROUP"},
		{"Arrival.2016.BluRay-ONE", "Arrival.2016.BluRay-TWO"},
	} {
		if score := subtitleReleaseSimilarity(pair[0], pair[1]); score >= .8 {
			t.Errorf("unsafe release bypass: %v = %f", pair, score)
		}
	}
	if score := subtitleReleaseSimilarity("Arrival.2016.BluRay-GROUP", "Arrival.2016.BluRay-GROUP.srt"); score < .8 {
		t.Errorf("same release = %f", score)
	}
}

func TestSubtitleDialogueReferencePrefersRequestedMainAudio(t *testing.T) {
	tracks := []AudioFacts{
		{SourceIndex: 1, Language: "eng", Role: "commentary", Default: true},
		{SourceIndex: 2, Language: "eng", Role: "main", Default: true},
		{SourceIndex: 4, Language: "spa", Role: "main"},
	}
	if index, ok := subtitleDialogueTrack(tracks, "es"); !ok || index != 4 {
		t.Fatalf("Spanish main = %d, %v", index, ok)
	}
	if index, ok := subtitleDialogueTrack(tracks, "fr"); !ok || index != 2 {
		t.Fatalf("default main = %d, %v", index, ok)
	}
	if _, ok := subtitleDialogueTrack(tracks[:1], "en"); ok {
		t.Fatal("commentary was selected")
	}
}

func TestSubtitleConvertersRetainOriginalAndDialogue(t *testing.T) {
	for name, input := range map[string]string{
		"ass":  "[Script Info]\nScriptType: v4.00+\n[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0:00:01.25,0:00:03.50,Default,Hello\\Nworld, again!",
		"ttml": `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="1.250s" end="3.500s">Hello<br/>world, again!</p></div></body></tt>`,
	} {
		t.Run(name, func(t *testing.T) {
			cleaned, err := cleanSubtitle([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if string(cleaned.Original) != input || len(cleaned.Cues) != 1 || cleaned.Cues[0].Start != 1250*time.Millisecond || !strings.Contains(string(cleaned.Data), "Hello\nworld, again!") {
				t.Fatalf("conversion = %#v", cleaned)
			}
		})
	}
}

func TestSubtitleConvertersRejectAmbiguousOrUnsupportedTiming(t *testing.T) {
	for _, input := range []string{
		"[Script Info]\n[Events]\nDialogue: 0,0:00:01.00,0:00:03.00,Hi",
		"[Script Info]\n[Events]\nFormat: Start, Start, End, Text\nDialogue: 0:00:01.00,0:00:01.00,0:00:03.00,Hi",
		`<tt><body begin="1s"><p begin="2s" end="3s">Hi</p></body></tt>`,
		`<tt><p begin="2s" end="1s">Hi</p></tt>`,
		`<tt><p begin="2s" end="3s" dur="1s">Hi</p></tt>`,
		`<tt><p begin="NaNs" end="3s">Hi</p></tt>`,
		`<?xml version="1.0"?><!DOCTYPE tt [<!ENTITY x SYSTEM "file:///etc/passwd">]><tt><p begin="1s" end="2s">&x;</p></tt>`,
	} {
		if _, err := cleanSubtitle([]byte(input)); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestSubtitleCleanupPreservesSpokenURLsAndRepeatedDialogue(t *testing.T) {
	input := "1\n00:00:01,000 --> 00:00:02,000\nVisit https://example.test now.\n\n2\n00:00:02,000 --> 00:00:03,000\nWait!\n\n3\n00:00:03,050 --> 00:00:04,000\nWait!\n"
	cleaned, err := cleanSubtitle([]byte(input))
	if err != nil || len(cleaned.Cues) != 3 || cleaned.Duplicates != 0 {
		t.Fatalf("dialogue changed: %#v, %v", cleaned, err)
	}
}

func TestSubtitleOriginalArchiveIsImmutableAndBounded(t *testing.T) {
	dir := t.TempDir()
	provider := &subtitleProvider{cache: dir, ledger: newSubtitleLedger(dir)}
	data := []byte("original subtitle bytes")
	var record subtitleRecord
	if err := provider.retainSubtitleOriginal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.OriginalFingerprint != subtitleFingerprint(data) {
		t.Fatal("original was not identified")
	}
	if err := provider.retainSubtitleOriginal(data, &record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider.originalPath(record.OriginalFingerprint), []byte("unexpected edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := provider.retainSubtitleOriginal(data, &record); err == nil {
		t.Fatal("corrupted original was overwritten")
	}
	before, _ := os.ReadDir(filepath.Join(dir, "subtitle-originals"))
	for _, bad := range [][]byte{nil, make([]byte, (4<<20)+1)} {
		if err := provider.retainSubtitleOriginal(bad, &record); err == nil {
			t.Fatal("accepted invalid original")
		}
	}
	after, _ := os.ReadDir(filepath.Join(dir, "subtitle-originals"))
	if len(before) != len(after) {
		t.Fatal("invalid input wrote an archive")
	}
}

func TestSubtitleEncodingOverrideAndTraditionalChineseDefault(t *testing.T) {
	input := []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n")
	cleaned, err := convertSubtitle(input, "en", subtitleConversionOptions{Encoding: "windows-1252"})
	if err != nil || !strings.Contains(string(cleaned.Data), "Café") || string(cleaned.Original) != string(input) {
		t.Fatalf("encoding = %q/%v", cleaned.Data, err)
	}
	if _, err = convertSubtitle(input, "en", subtitleConversionOptions{Encoding: "utf-8"}); err == nil {
		t.Fatal("invalid explicit UTF-8 accepted")
	}
	if subtitleLanguageEncoding("zh-Hant") != "big5" || subtitleOCRLanguage("zh-Hant") != "chi_tra" {
		t.Fatal("traditional Chinese selected simplified encoding or OCR")
	}
	for _, input := range []string{
		"[Script Info]\n[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:03.00,{\\p2}m 0 0 l 10 10",
		"1\n00:00:01,000 --> 00:00:02,000\n" + strings.Repeat("word ", 1<<20),
	} {
		if _, err := convertSubtitle([]byte(input), "en", subtitleConversionOptions{}); err == nil {
			t.Fatal("unsupported drawing or oversized conversion accepted")
		}
	}
}

func TestSubtitleSearchSeparatesFilenameMetadataFromMovieIdentity(t *testing.T) {
	for _, test := range []struct{ file, title, expectedTitle, expectedYear string }{
		{"Arrival.2016.1080p.BluRay-GROUP.mkv", "Arrival 2016 1080p BluRay GROUP", "Arrival", "2016"},
		{"1917.2019.BluRay-GROUP.mkv", "1917 2019 BluRay GROUP", "1917", "2019"},
		{"Class.of.1984.1982.BluRay-GROUP.mkv", "Class of 1984 1982 BluRay GROUP", "Class of 1984", "1982"},
		{"Arrival.2016.Extended.BluRay-GROUP.mkv", "Arrival 2016 Extended BluRay GROUP", "Arrival", "2016"},
		{"Arrival.BluRay-GROUP.mkv", "A curated title", "A curated title", ""},
		{"Class.of.1984.mkv", "Class of 1984", "Class of 1984", ""},
	} {
		title, year := subtitleSearchIdentity(library.Item{Title: test.title, Path: test.file})
		if title != test.expectedTitle || year != test.expectedYear {
			t.Errorf("%s = %q/%q", test.file, title, year)
		}
	}
}
