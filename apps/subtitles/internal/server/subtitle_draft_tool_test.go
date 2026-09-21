package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestSubtitleDraftCancellationMatchesEveryIdentityField(t *testing.T) {
	t.Parallel()
	input := subtitleDraftInput{DraftID: strings.Repeat("a", 64), Language: "en", Method: "ocr"}
	for _, field := range []string{"id", "item", "language", "method"} {
		t.Run(field, func(t *testing.T) {
			calls := 0
			draft := subtitleDraft{ID: input.DraftID, Item: "film", Language: "en", Method: "ocr", cancel: func() { calls++ }}
			draft = mismatchedDraft(field, draft)
			if _, status, err := draft.cancelRequest("film", input); err == nil || status != 409 || calls != 0 {
				t.Fatalf("mismatch caused cancellation: %d %v calls=%d", status, err, calls)
			}
		})
	}
}

func TestSubtitleDraftCancellationStopsRunningAndCompletedJob(t *testing.T) {
	input := subtitleDraftInput{DraftID: strings.Repeat("a", 64), Language: "en", Method: "ocr"}

	calls := 0
	draft := subtitleDraft{ID: input.DraftID, Item: "film", Language: "en", Method: "ocr", cancel: func() { calls++ }}
	result, status, err := draft.cancelRequest("film", input)
	if err != nil || status != 200 || result.State != "canceling" || calls != 1 {
		t.Fatalf("cancel: %#v %d %v calls=%d", result, status, err, calls)
	}
	draft.cancel = nil
	if _, status, err = draft.cancelRequest("film", input); err != nil || status != 200 || calls != 1 {
		t.Fatal("completed draft cancellation failed")
	}
}

func TestSubtitleOCRToolOnlySelectsMatchingEmbeddedBitmapTrack(t *testing.T) {
	t.Parallel()
	facts := probeResult{SubtitleFacts: []playback.SubtitleFacts{
		{SourceIndex: 0, Language: "en", Codec: "subrip", Text: true},
		{SourceIndex: 1, Language: "en", Codec: "dvd_subtitle", External: true},
		{SourceIndex: 2, Language: "en", Codec: "dvd_subtitle", Forced: true},
		{SourceIndex: -1, Language: "en", Codec: "dvd_subtitle"},
		{SourceIndex: 3, Language: "es", Codec: "dvd_subtitle"},
		{SourceIndex: 4, Language: "en", Codec: "unsupported"},
		{SourceIndex: 5, Language: "en", Codec: "hdmv_pgs_subtitle"},
	}}
	tool, err := selectSubtitleDraftTool(configuration.Snapshot{}, facts, subtitleDraftInput{Method: "ocr", Language: "en"})
	if err != nil || tool.stream != 5 || tool.language != "eng" || tool.executable != "tesseract" {
		t.Fatalf("tool: %#v %v", tool, err)
	}
	for _, language := range []string{"xx", "fr"} {
		if _, err := selectSubtitleOCRTool(configuration.Snapshot{}, facts, language); err == nil {
			t.Fatalf("unsupported track %s accepted", language)
		}
	}
}

func TestSubtitleTranscriptionSelectsMatchingDefaultMainAudio(t *testing.T) {
	t.Parallel()
	facts := probeResult{AudioFacts: []playback.AudioFacts{
		{SourceIndex: 0, Language: "en", Role: "commentary", Default: true},
		{SourceIndex: -1, Language: "en", Role: "main"},
		{SourceIndex: 1, Language: "es", Role: "main"},
		{SourceIndex: 2, Language: "en", Role: "main"},
		{SourceIndex: 3, Language: "en", Role: "main", Default: true},
		{SourceIndex: 4, Language: "en", Role: "main"},
	}}
	for language, want := range map[string]int{"en": 3, "es": 1, "fr": -1} {
		if got := subtitleTranscriptionStream(facts, language); got != want {
			t.Fatalf("%s track=%d want=%d", language, got, want)
		}
	}
}

func TestSubtitleTranscriptionToolRequiresModelAndMatchingAudio(t *testing.T) {
	t.Parallel()
	facts := probeResult{AudioFacts: []playback.AudioFacts{{SourceIndex: 3, Language: "en", Role: "main", Default: true}}}

	model := filepath.Join(t.TempDir(), "model.bin")
	if err := os.WriteFile(model, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_TRANSCRIPTION_MODEL" {
			return model, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	// Verify the configured model was actually consumed before exercising selection.
	if config.String("subtitles.transcription_model") != model {
		t.Fatal("model fixture did not configure transcription")
	}
	tool, err := selectSubtitleDraftTool(config, facts, subtitleDraftInput{Method: "transcription", Language: "en"})
	if err != nil || tool.stream != 3 || tool.model != model {
		t.Fatalf("tool: %#v %v", tool, err)
	}
	if _, err := selectSubtitleTranscriptionTool(config, facts, "fr"); err == nil {
		t.Fatal("unmatched audio accepted")
	}
	if _, err := selectSubtitleTranscriptionTool(configuration.Snapshot{}, facts, "en"); err == nil {
		t.Fatal("missing model accepted")
	}
}

func TestSubtitleLanguageEncodingAndRepeatedCueBoundaries(t *testing.T) {
	t.Parallel()
	for language, want := range map[string]string{"zh-Hant": "big5", "zh-TW": "big5", "zh-HK": "big5", "ru": "windows-1251", "pl": "windows-1250", "el": "windows-1253", "tr": "windows-1254", "he": "windows-1255", "ar": "windows-1256", "th": "windows-874", "ja": "shift-jis", "zh": "gb18030", "ko": "euc-kr", "en": "windows-1252"} {
		if got := subtitleLanguageEncoding(language); got != want {
			t.Fatalf("%s=%s want %s", language, got, want)
		}
	}
	cues := []subtitleCue{{Start: time.Second, End: 2 * time.Second, Text: "Hello"}, {Start: 2 * time.Second, End: 3 * time.Second, Text: "Hello"}, {Start: 5 * time.Second, End: 6 * time.Second, Text: "Hello"}, {Start: 6 * time.Second, End: 7 * time.Second, Text: "World"}}
	got, removed := mergeRepeatedSubtitleCues(cues)
	if removed != 1 || len(got) != 3 || got[0].End != 3*time.Second || got[1].Start != 5*time.Second {
		t.Fatalf("merged: %#v removed=%d", got, removed)
	}
}

func mismatchedDraft(field string, draft subtitleDraft) subtitleDraft {
	switch field {
	case "id":
		draft.ID = "other"
	case "item":
		draft.Item = "other"
	case "language":
		draft.Language = "es"
	case "method":
		draft.Method = "transcription"
	}
	return draft
}
