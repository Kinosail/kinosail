package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func subtitleProcessFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tool")
	writeSubtitleFactsExecutable(t, path, "#!/bin/sh\n"+body+"\n")
	return path
}

func TestLocalSubtitleTranscriptionProcessResults(t *testing.T) {
	t.Parallel()
	ffmpeg := subtitleProcessFixture(t, "exit 0")
	transcript := `{"result":{"language":"en"},"transcription":[{"offsets":{"from":1000,"to":3000},"text":"Hello world","tokens":[{"text":" Hello","p":0.95,"t_dtw":110,"offsets":{"from":1000,"to":2000}},{"text":" world","p":0.5,"t_dtw":220,"offsets":{"from":2000,"to":3000}}]}]}`
	whisper := subtitleProcessFixture(t, `while [ "$#" -gt 0 ]; do
if [ "$1" = --output-file ]; then shift; output="$1"; fi
shift
done
printf '%s' '`+transcript+`' > "$output.json"`)
	document, words, err := localSubtitleTranscription(t.Context(), ffmpeg, whisper, "media", 0, "en", "model", "small", 10)
	if err != nil || len(document.Cues) != 1 || len(words) != 2 || document.TimingEvidence != "unverified" {
		t.Fatalf("transcript=%#v words=%d err=%v", document, len(words), err)
	}
	fail := subtitleProcessFixture(t, "exit 1")
	for _, tools := range [][2]string{{fail, whisper}, {ffmpeg, fail}, {ffmpeg, ffmpeg}} {
		if _, _, err := localSubtitleTranscription(t.Context(), tools[0], tools[1], "media", 0, "en", "model", "small", 10); err == nil {
			t.Fatal("failed process or absent output accepted")
		}
	}
}

func TestLocalSubtitleOCRProcessProducesReviewableDraft(t *testing.T) {
	t.Parallel()
	ffmpeg := subtitleProcessFixture(t, "printf 'n: 0 pts: 0 pts_time:1\\n' >&2\ndd if=/dev/zero bs=2073600 count=1 2>/dev/null")
	tesseract := subtitleProcessFixture(t, "cat >/dev/null\nprintf 'level\\tpage_num\\tblock_num\\tpar_num\\tline_num\\tword_num\\tleft\\ttop\\twidth\\theight\\tconf\\ttext\\n5\\t1\\t1\\t1\\t1\\t1\\t0\\t0\\t20\\t10\\t95\\tHello\\n'")
	document, words, err := localSubtitleOCR(t.Context(), ffmpeg, tesseract, "media", 0, "eng", 10)
	if err != nil || len(document.Cues) != 1 || len(words) != 1 || document.Cues[0].Text != "Hello" || document.TimingEvidence != "unverified" {
		t.Fatalf("OCR=%#v words=%d err=%v", document, len(words), err)
	}
}

func TestLocalSubtitleOCRProcessRejectsInvalidFramesAndExit(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"failed": "exit 1", "empty": "exit 0", "partial": "printf x", "invalid time": "printf 'n: 0 pts: 0 pts_time:999\\n' >&2\nprintf x", "ignored log": "printf 'ordinary diagnostic\\n' >&2",
	} {
		t.Run(name, func(t *testing.T) {
			tool := subtitleProcessFixture(t, body)
			if _, _, err := localSubtitleOCR(t.Context(), tool, "missing", "media", 0, "eng", 10); err == nil {
				t.Fatal("invalid OCR output accepted")
			}
		})
	}
	if _, _, err := localSubtitleOCR(t.Context(), filepath.Join(t.TempDir(), "missing"), "missing", "media", 0, "eng", 10); err == nil {
		t.Fatal("missing executable accepted")
	}
}

func TestSubtitleGenerationFailureAndCancellationRetainNoDocument(t *testing.T) {
	t.Parallel()
	media := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := library.Item{ID: "0123456789abcdef", Path: media}
	version, err := subtitleMediaVersion(item)
	if err != nil {
		t.Fatal(err)
	}
	fail := subtitleProcessFixture(t, "exit 1")
	for _, method := range []string{"ocr", "transcription"} {
		for _, canceled := range []bool{false, true} {
			t.Run(method+map[bool]string{true: " canceled", false: " failed"}[canceled], func(t *testing.T) {
				assertFailedDraftGeneration(t, method, canceled, item, version, fail)
			})
		}
	}
}

func assertFailedDraftGeneration(t *testing.T, method string, canceled bool, item library.Item, version, fail string) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if canceled {
		cancel()
	}
	draft := subtitleDraft{ID: strings.Repeat("a", 64), Method: method, version: version, cancel: cancel}
	manager := &subtitleManager{probe: &mediaProbe{ffmpeg: fail}, drafts: subtitleDraftStore{current: draft}}
	manager.generateSubtitleDraft(ctx, item, draft, 0, "en", fail, "model", "small", 10)
	want := "failed"
	if canceled {
		want = "stopped"
	}
	if manager.drafts.current.State != want || manager.drafts.current.Document != nil || manager.drafts.current.cancel != nil {
		t.Fatalf("generation state=%#v", manager.drafts.current)
	}
}
