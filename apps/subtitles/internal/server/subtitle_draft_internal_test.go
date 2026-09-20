package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleDraftValidationPrecedesAllDependencies(t *testing.T) {
	manager := &subtitleManager{}
	for _, input := range []subtitleDraftInput{
		{}, {Language: "en", Method: "ocr", Action: "unknown"}, {Language: "en", Method: "remote", Action: "start"},
		{Language: "../en", Method: "ocr", Action: "start"}, {Language: "en", Method: "ocr", Action: "start", DraftID: strings.Repeat("a", 64)},
		{Language: "en", Method: "ocr", Action: "cancel"}, {Language: "en", Method: "ocr", Action: "cancel", DraftID: strings.Repeat("x", 65)},
	} {
		_, status, err := manager.localSubtitleDraft(httptest.NewRequest(http.MethodPost, "/", nil), "0123456789abcdef", input)
		if err == nil || status != http.StatusBadRequest {
			t.Errorf("accepted invalid draft: %+v", input)
		}
	}
	if manager.drafts.current.ID != "" {
		t.Fatal("rejected input started a job")
	}
}

func TestSubtitleDraftIsBoundToItsVideoLanguageAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(path, []byte("first video"), 0600); err != nil {
		t.Fatal(err)
	}
	item := library.Item{ID: "0123456789abcdef", Path: path}
	version, err := subtitleMediaVersion(item)
	if err != nil {
		t.Fatal(err)
	}
	manager := &subtitleManager{}
	manager.drafts.current = subtitleDraft{ID: strings.Repeat("a", 64), Item: item.ID, Language: "en", Method: "ocr", State: "ready", version: version, data: []byte("draft")}
	data, source, err := manager.subtitleDraftData(item, "en", strings.Repeat("a", 64))
	if err != nil || string(data) != "draft" || source != "ocr" {
		t.Fatalf("draft=%s/%s/%v", data, source, err)
	}
	data[0] = 'X'
	if string(manager.drafts.current.data) != "draft" {
		t.Fatal("caller changed retained draft")
	}
	for _, pair := range [][2]string{{"es", strings.Repeat("a", 64)}, {"en", strings.Repeat("b", 64)}} {
		if _, _, err = manager.subtitleDraftData(item, pair[0], pair[1]); err == nil {
			t.Fatal("accepted mismatched draft")
		}
	}
	if err = os.WriteFile(path, []byte("replacement video is different"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = manager.subtitleDraftData(item, "en", strings.Repeat("a", 64)); err == nil {
		t.Fatal("accepted changed video")
	}
}

func TestSubtitleOCRParsesConfidenceAndLines(t *testing.T) {
	input := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"5\t1\t1\t1\t1\t1\t0\t0\t20\t10\t95.5\tHello\n" +
		"5\t1\t1\t1\t1\t2\t0\t0\t20\t10\t70\tworld\n" +
		"5\t1\t1\t1\t2\t1\t0\t0\t20\t10\t90\tAgain\n"
	text, words, err := parseSubtitleOCR([]byte(input))
	if err != nil || text != "Hello world\nAgain" || len(words) != 3 || words[1].Confidence != .7 {
		t.Fatalf("OCR=%q/%+v/%v", text, words, err)
	}
	for _, bad := range []string{"", "not tsv", strings.Replace(input, "95.5", "NaN", 1), strings.Replace(input, "95.5", "101", 1), strings.Replace(input, "95.5", "-1", 1), strings.Repeat("x", (1<<20)+1)} {
		if _, _, err := parseSubtitleOCR([]byte(bad)); err == nil {
			t.Fatal("accepted malformed OCR")
		}
	}
	pixels := bytes.Repeat([]byte{255}, 100*100)
	if image := subtitleOCRImage(pixels, 100, 100); image != nil {
		t.Fatal("blank frame became text")
	}
	pixels[50*100+50] = 0
	image := subtitleOCRImage(pixels, 100, 100)
	if !bytes.HasPrefix(image, []byte("P5\n41 41\n255\n")) {
		t.Fatalf("crop header = %q", image[:min(len(image), 20)])
	}
}

func TestSubtitleOCRBlankFramesProduceNoInstallableDocument(t *testing.T) {
	frames := bytes.NewReader(bytes.Repeat([]byte{255}, 1920*1080))
	times := make(chan float64, 1)
	times <- 0
	close(times)
	cues, _, err := readSubtitleOCRFrames(context.Background(), frames, times, "must-not-run", "eng", 10)
	if err != nil || len(cues) != 0 {
		t.Fatalf("blank OCR=%v/%v", cues, err)
	}
	if _, err = subtitleDocument(cues, 0); err == nil {
		t.Fatal("blank draft became installable")
	}
}

func TestSubtitleTranscriptKeepsLocalWordTimingAndRejectsInvalidOutput(t *testing.T) {
	input := `{"result":{"language":"en"},"transcription":[{"offsets":{"from":1000,"to":3000},"text":"Hello world","tokens":[{"text":" Hello","p":0.95,"t_dtw":110,"offsets":{"from":1000,"to":2000}},{"text":" world","p":0.5,"t_dtw":220,"offsets":{"from":2000,"to":3000}}]}]}`
	document, words, err := parseSubtitleTranscript([]byte(input), "en", 10)
	if err != nil || len(document.Cues) != 1 || len(words) != 2 || words[0].Start != 1.1 || words[1].Confidence != .5 || !bytes.Equal(document.Original, document.Data) {
		t.Fatalf("transcript=%+v/%+v/%v", document, words, err)
	}
	for _, bad := range []string{"{}", strings.Replace(input, `"en"`, `"es"`, 1), strings.Replace(input, `"to":3000`, `"to":1000`, 1), strings.Replace(input, `"p":0.95`, `"p":1.1`, 1), strings.Replace(input, `"from":1000`, `"from":-1`, 1), strings.Replace(input, `"to":3000`, `"to":13000`, 1)} {
		if _, _, err := parseSubtitleTranscript([]byte(bad), "en", 10); err == nil {
			t.Errorf("accepted invalid transcript: %s", bad)
		}
	}
}

func TestContinuousSubtitleDriftAndIndependentValidation(t *testing.T) {
	anchors := []subtitleAlignmentAnchor{{At: time.Minute, Offset: time.Second}, {At: 31 * time.Minute, Offset: 2800 * time.Millisecond}, {At: 61 * time.Minute, Offset: 4600 * time.Millisecond}}
	fitted, ok := continuousSubtitleAlignment(subtitleAlignment{Scale: 1}, anchors)
	if !ok || fitted.Scale != 1.001 || absDuration(fitted.Offset-940*time.Millisecond) > time.Millisecond {
		t.Fatalf("continuous fit=%+v/%v", fitted, ok)
	}
	anchors[1].Offset += time.Second
	if _, ok = continuousSubtitleAlignment(subtitleAlignment{Scale: 1}, anchors); ok {
		t.Fatal("nonlinear drift accepted as linear")
	}
	if !subtitleAbruptTimingChange([]subtitleAlignmentAnchor{{Offset: 0}, {Offset: 6 * time.Second}}) {
		t.Fatal("large cut change accepted")
	}
	duration := 30 * time.Minute
	cues := []subtitleCue{}
	for at := 2 * time.Second; at < duration-10*time.Second; at += 17 * time.Second {
		cues = append(cues, subtitleCue{Start: at, End: at + 3*time.Second, Text: "Dialogue"})
	}
	audio := subtitleActivity(cues, 1, subtitleFrame, int(duration/subtitleFrame))
	if !validateSubtitleRegions(audio, cues) {
		t.Fatal("aligned full-length dialogue rejected")
	}
	shifted := append([]subtitleCue(nil), cues...)
	for i := range shifted {
		if shifted[i].Start > 20*time.Minute {
			shifted[i].Start += 4 * time.Second
			shifted[i].End += 4 * time.Second
		}
	}
	if validateSubtitleRegions(audio, shifted) {
		t.Fatal("late holdout mismatch accepted")
	}
	if validateSubtitleRegions(audio, []subtitleCue{{Start: duration, End: duration + 3*time.Second, Text: "Beyond audio"}}) {
		t.Fatal("truncated analysis accepted")
	}
}

func TestSubtitleEditAnchorsAndCorrectedTextValidation(t *testing.T) {
	for _, input := range []subtitleEdit{
		{Language: "en", Text: strings.Repeat("x", (4<<20)+1)},
		{Language: "en", DraftID: "invalid"},
		{Language: "en", DraftID: strings.Repeat("a", 64), Data: []byte("conflict")},
		{Language: "en", Offset: 1, Anchors: []subtitleEditAnchor{{At: 1, Offset: 0}}},
		{Language: "en", Anchors: []subtitleEditAnchor{{At: 1000, Offset: 0}, {At: 2000, Offset: -1000}}},
		{Language: "en", AutomaticSync: true, Offset: 1},
	} {
		if validateSubtitleEdit(input, false) == nil {
			t.Errorf("accepted invalid edit: %+v", input.Anchors)
		}
	}
	var anchor subtitleEditAnchor
	for _, raw := range []string{`{}`, `{"atMilliseconds":1}`, `{"atMilliseconds":1,"offsetMilliseconds":null}`, `{"atMilliseconds":1,"offsetMilliseconds":0,"other":1}`} {
		if json.Unmarshal([]byte(raw), &anchor) == nil {
			t.Errorf("accepted anchor %s", raw)
		}
	}
}

func TestGeneratedDialogueDoesNotSatisfyCaptionCoverage(t *testing.T) {
	directory := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(directory, "Film.mp4")}
	data := []byte("1\n00:00:01,000 --> 00:00:03,000\nDialogue\n")
	target := subtitleSidecarPath(item, "en")
	item.Subtitles = []string{target}
	if err := os.WriteFile(target, data, 0600); err != nil {
		t.Fatal(err)
	}
	settings := newSettingsStore(directory, t.TempDir(), "", nil)
	settings.value.SubtitlePreference = "sdh"
	provider := &subtitleProvider{ledger: newSubtitleLedger(t.TempDir())}
	manager := &subtitleManager{settings: settings, provider: provider}
	record := completeSubtitleRecord(target, data, subtitleRecord{Source: "transcription", Role: "translation", Managed: true, Frozen: true, Synchronization: "none"})
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil {
		t.Fatal(err)
	}
	if ready, _ := manager.sidecarPlanCoverage(item, "en"); ready {
		t.Fatal("generated dialogue counted as captions")
	}
	record.Role = "captions"
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil {
		t.Fatal(err)
	}
	if ready, _ := manager.sidecarPlanCoverage(item, "en"); !ready {
		t.Fatal("explicitly reviewed captions did not satisfy coverage")
	}
	if err := os.WriteFile(target, []byte("changed dialogue"), 0600); err != nil {
		t.Fatal(err)
	}
	if ready, _ := manager.sidecarPlanCoverage(item, "en"); ready {
		t.Fatal("changed file kept stale caption evidence")
	}
}

func TestSubtitleRecoveryKeepsBothFormatOriginals(t *testing.T) {
	directory := t.TempDir()
	provider := &subtitleProvider{ledger: newSubtitleLedger(t.TempDir())}
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(directory, "Film.mp4")}
	provider.index = sidecarTestIndex(item)
	first, _ := cleanSubtitle([]byte("[Script Info]\n[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:02.00,First\n"))
	second, _ := cleanSubtitle([]byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nSecond\n"))
	target := subtitleSidecarPath(item, "en")
	old := subtitleRecord{Fingerprint: subtitleFingerprint(first.Data), Role: "captions"}
	if err := provider.retainSubtitleOriginal(first.Original, &old); err != nil {
		t.Fatal(err)
	}
	next := subtitleRecord{Source: "external", Role: "translation", Frozen: true, Backup: true, Synchronization: "none"}
	if err := provider.retainSubtitleOriginal(second.Original, &next); err != nil {
		t.Fatal(err)
	}
	if err := provider.retainSubtitleRecoveryOriginal(first.Data, old, &next); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, second.Data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".kinosail.bak", first.Data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), completeSubtitleRecord(target, second.Data, next)); err != nil {
		t.Fatal(err)
	}
	if err := provider.restorePrevious(item, "en"); err != nil {
		t.Fatal(err)
	}
	restored, _, _ := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if restored.OriginalFingerprint != old.OriginalFingerprint || restored.BackupOriginalFingerprint != next.OriginalFingerprint || restored.Role != "captions" || !restored.Frozen {
		t.Fatalf("restored provenance=%+v", restored)
	}
	if err := provider.restorePrevious(item, "en"); err != nil {
		t.Fatal(err)
	}
	redone, _, _ := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if redone.OriginalFingerprint != next.OriginalFingerprint || redone.Role != "translation" {
		t.Fatalf("redo provenance=%+v", redone)
	}
}
