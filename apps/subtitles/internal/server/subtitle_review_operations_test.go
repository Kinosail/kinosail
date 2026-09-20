package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func subtitleReviewFixture(t *testing.T) (*subtitleManager, library.Item, *http.Request, []byte) {
	t.Helper()
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"10\"}}'\n")
	manager.index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
	data := []byte("1\n00:00:01,000 --> 00:00:03,000\nHello world\n")
	if err := os.WriteFile(subtitleSidecarPath(item, "en"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), "POST", "/", nil)
	return manager, item, request, data
}

func TestSubtitleReviewExplainsMatchingHistoryAndExportsOriginal(t *testing.T) {
	t.Parallel()
	manager, item, request, data := subtitleReviewFixture(t)
	for _, record := range []subtitleRecord{
		{Source: "subdl", TimingEvidence: "file-hash"}, {Source: "embedded"}, {Source: "ocr"}, {Source: "subdl", ReleaseMatch: .9}, {Source: "subdl", ReleaseMatch: .5}, {Source: "external"},
	} {
		record.Fingerprint = subtitleFingerprint(data)
		if err := manager.provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil {
			t.Fatal(err)
		}
		review, status, err := manager.inspectSubtitle(request, item.ID, "en")
		if err != nil || status != 200 || review.Current == nil || review.Source != record.Source || review.MatchEvidence == "No verified identity evidence" {
			t.Fatalf("review=%#v %d %v", review, status, err)
		}
	}
}

func TestSubtitleExportKeepsDialogueInRequestedFormat(t *testing.T) {
	t.Parallel()
	manager, item, request, _ := subtitleReviewFixture(t)

	for format, mime := range map[string]string{"srt": "application/x-subrip; charset=utf-8", "vtt": "text/vtt; charset=utf-8"} {
		result, kind, status, err := manager.exportSubtitle(request, item.ID, "en", format)
		if err != nil || status != 200 || kind != mime || !strings.Contains(string(result), "Hello world") {
			t.Fatalf("export %s=%q %s %d %v", format, result, kind, status, err)
		}
	}
}

func TestSubtitleOriginalExportVerifiesRetainedBytes(t *testing.T) {
	t.Parallel()
	manager, item, request, data := subtitleReviewFixture(t)

	record := subtitleRecord{Source: "external", Fingerprint: subtitleFingerprint(data), OriginalFingerprint: subtitleFingerprint(data)}
	original := manager.provider.originalPath(record.OriginalFingerprint)
	installReviewOriginal(t, manager, item, record, data)
	result, kind, status, err := manager.exportSubtitle(request, item.ID, "en", "original")
	if err != nil || status != 200 || kind != "application/octet-stream" || string(result) != string(data) {
		t.Fatalf("original=%q %s %d %v", result, kind, status, err)
	}
	if err := os.WriteFile(original, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, status, err := manager.exportSubtitle(request, item.ID, "en", "original"); status != 409 || err == nil {
		t.Fatal("tampered original exported")
	}
}

func TestSubtitlePreviewRejectsStaleAndInvalidEditsWithoutWriting(t *testing.T) {
	t.Parallel()
	manager, item, request, data := subtitleReviewFixture(t)
	for _, input := range []subtitleEdit{
		{Language: "en", Fingerprint: strings.Repeat("a", 64)},
		{Language: "en", DraftID: strings.Repeat("b", 64)},
		{Language: "en", Data: []byte("invalid")},
		{Language: "en", Text: "invalid"},
		{Language: "en", Offset: -120000},
	} {
		if _, _, status, err := manager.previewSubtitleEdit(request, item.ID, input); err == nil || status < 400 {
			t.Fatalf("invalid edit accepted: %#v %d %v", input, status, err)
		}
		got, err := os.ReadFile(subtitleSidecarPath(item, "en"))
		if err != nil || string(got) != string(data) {
			t.Fatal("preview changed sidecar")
		}
	}
}

func TestReviewedDraftApplyRetainsOriginalAndRejectsStaleSave(t *testing.T) {
	t.Parallel()
	manager, _, request, current := subtitleReviewFixture(t)
	item := scannedReviewItem(t, manager)
	generated := []byte("1\n00:00:01,000 --> 00:00:03,000\nRecognized words\n")
	draftID := strings.Repeat("d", 64)
	installReadyReviewDraft(t, manager, item, draftID, generated)
	input := subtitleEdit{Language: "en", Fingerprint: subtitleFingerprint(current), DraftID: draftID, Text: "1\n00:00:01,000 --> 00:00:03,000\nReviewed words\n"}
	assertReviewedDraftSaved(t, manager, item, request, input, generated)
	if _, status, err := manager.applySubtitleEdit(request, item.ID, input); status != 409 || err == nil {
		t.Fatalf("stale save=%d %v", status, err)
	}
}

func scannedReviewItem(t *testing.T, manager *subtitleManager) library.Item {
	t.Helper()
	if err := manager.index.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	items, err := manager.index.Snapshot()
	if err != nil || len(items) != 1 {
		t.Fatalf("scanned items=%d %v", len(items), err)
	}
	return items[0]
}

func installReadyReviewDraft(t *testing.T, manager *subtitleManager, item library.Item, draftID string, generated []byte) {
	t.Helper()
	version, err := subtitleMediaVersion(item)
	if err != nil {
		t.Fatal(err)
	}
	manager.drafts.current = subtitleDraft{ID: draftID, Item: item.ID, Language: "en", Method: "transcription", State: "ready", version: version, data: generated}
}

func assertReviewedDraftSaved(t *testing.T, manager *subtitleManager, item library.Item, request *http.Request, input subtitleEdit, generated []byte) {
	t.Helper()
	review, status, err := manager.applySubtitleEdit(request, item.ID, input)
	if err != nil || status != 200 || review.Source != "transcription" || !review.Restorable {
		t.Fatalf("applied review=%#v %d %v", review, status, err)
	}
	saved, err := os.ReadFile(subtitleSidecarPath(item, "en"))
	if err != nil || !strings.Contains(string(saved), "Reviewed words") {
		t.Fatalf("saved=%q %v", saved, err)
	}
	original, _, status, err := manager.exportSubtitle(request, item.ID, "en", "original")
	if err != nil || status != 200 || string(original) != string(generated) {
		t.Fatalf("original=%q %d %v", original, status, err)
	}
}

func installReviewOriginal(t *testing.T, manager *subtitleManager, item library.Item, record subtitleRecord, data []byte) {
	t.Helper()
	original := manager.provider.originalPath(record.OriginalFingerprint)

	if err := os.MkdirAll(filepath.Dir(original), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil {
		t.Fatal(err)
	}
}
