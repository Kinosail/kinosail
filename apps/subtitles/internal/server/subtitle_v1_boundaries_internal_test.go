package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	xunicode "golang.org/x/text/encoding/unicode"
)

func TestSubtitleEncodingAndIdentifierBoundaries(t *testing.T) { //nolint:cyclop // One table proves every public v1 encoding and identifier boundary.
	t.Parallel()
	for name, input := range map[string][]byte{
		"empty":          nil,
		"oversized":      bytes.Repeat([]byte("x"), (4<<20)+1),
		"UTF-8 BOM only": {0xef, 0xbb, 0xbf},
		"UTF-16 partial": {0xff, 0xfe, 0x00},
		"NUL":            []byte("subtitle\x00text"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSubtitleText(input); err == nil {
				t.Fatal("invalid subtitle encoding was accepted")
			}
		})
	}
	if _, err := subtitleDecodedText([]byte{0xff, 0xfe, 0x00}, xunicode.UTF16(xunicode.LittleEndian, xunicode.ExpectBOM).NewDecoder()); err == nil {
		t.Fatal("truncated UTF-16 was accepted")
	}
	if threeDigits(0) != "000" || threeDigits(9) != "009" || threeDigits(99) != "099" || threeDigits(100) != "100" {
		t.Fatal("subtitle millisecond formatting is invalid")
	}
	for _, invalid := range []string{"", "0123456789abcde", "0123456789abcdeg", "0123456789ABCDE"} {
		if validSubtitleItemID(invalid) {
			t.Fatalf("invalid subtitle item ID %q was accepted", invalid)
		}
	}
}

func TestSubtitleDirectoryWritableUsesPortableProbe(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if !subtitleDirectoryWritable(directory) {
		t.Fatal("writable directory was rejected")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("write probe left files: %v, %v", entries, err)
	}
	if subtitleDirectoryWritable(filepath.Join(directory, "missing")) {
		t.Fatal("missing directory was writable")
	}
}

func TestSubtitleProviderHealthAuthenticationDateAndPersistenceBoundaries(t *testing.T) { //nolint:cyclop,gocognit // One test covers bounded authentication, date, and persisted-state failures.
	t.Parallel()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	registry := newSubtitleProviderHealthRegistry()
	registry.now = func() time.Time { return now }
	registry.observe("OpenSubtitles", &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}, nil)
	view := registry.views(map[string]bool{"OpenSubtitles": true})[1]
	if view.LastSafeError != "Credentials rejected" || view.NextRetry != now.Add(24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("authentication health = %#v", view)
	}
	registry.observe("SubSource", &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}}, nil)
	if view = registry.views(map[string]bool{"SubSource": true})[2]; view.LastSafeError != "Provider unavailable" || view.NextRetry == "" {
		t.Fatalf("transient health = %#v", view)
	}
	registry.observe("SubDL", &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}, nil)
	if view = registry.views(map[string]bool{"SubDL": true})[0]; view.NextRetry != now.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("rate-limit fallback = %#v", view)
	}
	header := http.Header{"Retry-After": []string{now.Add(10 * time.Minute).Format(http.TimeFormat)}}
	if retry := subtitleRetryTime(header, now); !retry.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("HTTP-date retry = %v", retry)
	}
	if retry := subtitleRetryTime(http.Header{"Retry-After": []string{"invalid"}}, now); !retry.IsZero() {
		t.Fatalf("invalid retry date = %v", retry)
	}
	header = http.Header{}
	header.Set("X-RateLimit-Remaining", "17")
	remaining, _ := subtitleQuota(header, now, nil, time.Time{})
	if remaining == nil || *remaining != 17 {
		t.Fatalf("provider quota = %v", remaining)
	}
	header = http.Header{}
	header.Set("RateLimit-Reset", strconv.FormatInt(now.Add(time.Hour).Unix(), 10))
	if _, reset := subtitleQuota(header, now, nil, time.Time{}); !reset.Equal(now.Add(time.Hour)) {
		t.Fatalf("absolute quota reset = %v", reset)
	}

	for name, contents := range map[string]string{
		"unknown field": `{"version":1,"providers":{},"unknown":true}`,
		"bad version":   `{"version":2,"providers":{}}`,
		"bad provider":  `{"version":1,"providers":{"Other":{}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			data := t.TempDir()
			if err := os.WriteFile(filepath.Join(data, "subtitle_provider_health.json"), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			loaded := newSubtitleProviderHealthRegistry(data)
			if len(loaded.states) != 0 {
				t.Fatal("invalid provider state was loaded")
			}
		})
	}
}

func TestSubtitleAutomationStopsDuringCircuitDelay(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	settings := newSettingsStore(media, data, "", nil)
	index := newLibraryIndex(t.Context(), settings.roots(), t.TempDir(), time.Hour, nil)
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), data, index, settings, "")
	manager := newSubtitleManager(index, settings, provider, nil)
	if result := manager.automate(t.Context(), 0); result.Attempted != 0 {
		t.Fatalf("zero-limit automation = %#v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	trigger := make(chan struct{}, 2)
	done := make(chan struct{})
	go func() {
		manager.runAutomation(ctx, trigger)
		close(done)
	}()
	trigger <- struct{}{}
	trigger <- struct{}{}
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("automation did not stop during its delay")
	}
}

func TestSubtitleOperationsRejectMissingItemsAndEmptyLibraries(t *testing.T) { //nolint:cyclop // One empty index proves shared operation status and no-side-effect branches.
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	settings := newSettingsStore(media, data, "", nil)
	index := newLibraryIndex(t.Context(), settings.roots(), t.TempDir(), time.Hour, nil)
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), data, index, settings, "")
	manager := newSubtitleManager(index, settings, provider, nil)
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), http.MethodPost, "/", nil)
	if _, status, err := manager.subtitleItem(request, "bad"); status != http.StatusBadRequest || err == nil {
		t.Fatalf("invalid item = %d, %v", status, err)
	}
	if _, status, err := manager.subtitleItem(request, "ffffffffffffffff"); status != http.StatusNotFound || err == nil {
		t.Fatalf("missing item = %d, %v", status, err)
	}
	if status, err := manager.fetch(request, "bad", "en"); status != http.StatusBadRequest || err == nil {
		t.Fatalf("invalid fetch = %d, %v", status, err)
	}
	if status, err := manager.restore(request, "bad"); status != http.StatusBadRequest || err == nil {
		t.Fatalf("invalid restore = %d, %v", status, err)
	}
	if status, err := manager.setReplacement(request, "bad", false); status != http.StatusBadRequest || err == nil {
		t.Fatalf("invalid replacement = %d, %v", status, err)
	}
	if attempted, written, err := manager.fetchWanted(request, "en", 10); attempted != 0 || written != 0 || err != nil {
		t.Fatalf("empty fetch = %d, %d, %v", attempted, written, err)
	}
	if result, err := manager.maintain(request, "en", 10); result.Attempted != 0 || err != nil {
		t.Fatalf("empty maintenance = %#v, %v", result, err)
	}
}

func TestSubtitleUpgradeEligibilityAndRecoveryRollback(t *testing.T) { //nolint:cyclop,gocognit // One fixture proves freeze, perfect score, recency, ledger failure, and restore rollback.
	t.Parallel()
	media := t.TempDir()
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Path: filepath.Join(media, "Movie.mp4")}
	target := subtitleSidecarPath(item, "en")
	current := []byte("1\n00:00:01,000 --> 00:00:02,000\nCurrent\n")
	previous := []byte("1\n00:00:01,000 --> 00:00:02,000\nPrevious\n")
	if err := os.WriteFile(target, current, 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: "https://example.com/api/v1", APIKey: "key"}, t.TempDir(), "", nil, nil, "")
	now := time.Now()
	record := completeSubtitleRecord(target, current, subtitleRecord{Fingerprint: subtitleFingerprint(current), Source: "external", CheckedAt: now.Unix(), InstalledAt: now.Unix(), Synchronization: "none", Frozen: true})
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil {
		t.Fatal(err)
	}
	if provider.upgradeEligible(item, "en", now) {
		t.Fatal("frozen subtitle was eligible")
	}
	record.Frozen, record.Managed, record.Score = false, true, 100
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil || provider.upgradeEligible(item, "en", now) {
		t.Fatal("perfect managed subtitle was eligible")
	}
	record.Score = 50
	if err := provider.ledger.store(subtitleRecordKey(item.ID, "en"), record); err != nil || provider.upgradeEligible(item, "en", now) {
		t.Fatal("recent managed subtitle was eligible")
	}
	provider.ledger.err = errors.New("unavailable")
	if provider.upgradeEligible(item, "en", now) {
		t.Fatal("subtitle with an unavailable ledger was eligible")
	}
	missing := item
	missing.ID, missing.Path = "fedcba9876543210", filepath.Join(media, "Missing.mp4")
	if upgraded, err := provider.upgradeSidecar(t.Context(), missing, "en"); upgraded || err == nil {
		t.Fatal("missing upgrade sidecar was accepted")
	}
	if upgraded, err := provider.upgradeSidecar(t.Context(), item, "en"); upgraded || err == nil {
		t.Fatal("upgrade with an unavailable ledger was accepted")
	}
	provider.ledger.err, provider.ledger.path = nil, t.TempDir()
	if err := os.WriteFile(target+".kinosail.bak", previous, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provider.restorePrevious(item, "en"); err == nil {
		t.Fatal("restore with failed ledger persistence was accepted")
	}
	restored, _ := os.ReadFile(target)
	backup, _ := os.ReadFile(target + ".kinosail.bak")
	if !bytes.Equal(restored, current) || !bytes.Equal(backup, previous) {
		t.Fatalf("restore rollback = %q, %q", restored, backup)
	}
}

func TestSubtitleOperationsReportProviderAndRecoveryFailures(t *testing.T) { //nolint:cyclop,gocognit // One scanned item proves conflict and upstream failure status mappings.
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	video := filepath.Join(media, "Movie.mp4")
	if err := os.WriteFile(video, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := newSettingsStore(media, data, "", nil)
	index := newLibraryIndex(t.Context(), settings.roots(), t.TempDir(), time.Hour, nil)
	if err := index.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	items, err := index.Snapshot()
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %d, %v", len(items), err)
	}
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), data, index, settings, "")
	manager := newSubtitleManager(index, settings, provider, nil)
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "owner", Owner: true}), http.MethodPost, "/", nil)
	if status, err := manager.fetch(request, items[0].ID, "en"); status != http.StatusBadGateway || err == nil {
		t.Fatalf("provider failure = %d, %v", status, err)
	}
	target := subtitleSidecarPath(items[0], "en")
	if err := os.WriteFile(target, []byte("1\n00:00:01,000 --> 00:00:02,000\nDialogue\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status, err := manager.fetch(request, items[0].ID, "en"); status != http.StatusConflict || err == nil {
		t.Fatalf("existing subtitle = %d, %v", status, err)
	}
	if status, err := manager.restore(request, items[0].ID); status != http.StatusConflict || err == nil {
		t.Fatalf("missing recovery copy = %d, %v", status, err)
	}
	if status, err := manager.setReplacement(request, items[0].ID, false); status != http.StatusNoContent || err != nil {
		t.Fatalf("replacement policy = %d, %v", status, err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if attempted, written, err := manager.fetchWanted(request, "en", 1); attempted != 1 || written != 0 || err == nil {
		t.Fatalf("wanted provider failure = %d, %d, %v", attempted, written, err)
	}
}

func TestSubtitleTimingParserRejectsAmbiguousBoundaries(t *testing.T) { //nolint:cyclop // One table proves timing syntax and ordering boundaries.
	t.Parallel()
	for _, value := range []string{
		"missing arrow",
		"00:00:01,000 -->",
		"bad --> 00:00:02,000",
		"00:00:02,000 --> 00:00:01,000",
		"00:60:01,000 --> 00:61:02,000",
	} {
		if _, _, ok := parseSubtitleTiming(value); ok {
			t.Fatalf("invalid timing %q was accepted", value)
		}
	}
	start, end, ok := parseSubtitleTiming("00:01.000 --> 00:02.000 align:start")
	if !ok || start != time.Second || end != 2*time.Second {
		t.Fatalf("WebVTT timing = %v, %v, %v", start, end, ok)
	}
}
