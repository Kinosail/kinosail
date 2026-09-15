package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/encoding/charmap"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func TestSubtitleCleanupAcceptsPublicV1TextEncodingsAndFormats(t *testing.T) {
	t.Parallel()
	srt := "1\r\n00:00:01,000 --> 00:00:02,000\r\nCafé\r\n"
	utf16, _, err := transform.Bytes(xunicode.UTF16(xunicode.LittleEndian, xunicode.UseBOM).NewEncoder(), []byte(srt))
	if err != nil {
		t.Fatal(err)
	}
	windows1252, _, err := transform.Bytes(charmap.Windows1252.NewEncoder(), []byte(srt))
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string][]byte{
		"utf-8":        []byte(srt),
		"utf-8 bom":    append([]byte{0xef, 0xbb, 0xbf}, []byte(srt)...),
		"utf-16 bom":   utf16,
		"windows-1252": windows1252,
		"webvtt":       []byte("WEBVTT\n\n00:01.000 --> 00:02.000\nCafé\n"),
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			cleaned, cleanErr := cleanSubtitle(input)
			if cleanErr != nil || !bytes.Contains(cleaned.Data, []byte("Café")) || bytes.HasPrefix(cleaned.Data, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(cleaned.Data, []byte("00:00:01,000 --> 00:00:02,000")) {
				t.Fatalf("cleaned = %q, %v", cleaned.Data, cleanErr)
			}
		})
	}
	invalid := append([]byte("1\n00:00:01,000 --> 00:00:02,000\n"), 0x81)
	if _, err = cleanSubtitle(invalid); err == nil {
		t.Fatal("undefined Windows-1252 byte was accepted")
	}
}

func TestSubtitleProviderHealthHonorsQuotaRetryAndCircuitState(t *testing.T) { //nolint:cyclop // One test proves the complete provider state transition.
	t.Parallel()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	registry := newSubtitleProviderHealthRegistry()
	registry.now = func() time.Time { return now }
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"120"}, "X-Ratelimit-Remaining": []string{"0"}}}
	registry.observe("SubDL", response, os.ErrDeadlineExceeded)
	view := registry.views(map[string]bool{"SubDL": true})[0]
	if view.State != "limited" || view.NextRetry != now.Add(2*time.Minute).Format(time.RFC3339) || view.Remaining == nil || *view.Remaining != 0 || registry.before("SubDL") == nil {
		t.Fatalf("limited view = %#v", view)
	}
	now = now.Add(2 * time.Minute)
	registry.observe("SubDL", &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, nil)
	view = registry.views(map[string]bool{"SubDL": true})[0]
	if view.State != "connected" || !view.Tested || registry.before("SubDL") != nil {
		t.Fatalf("recovered view = %#v", view)
	}
	for range 3 {
		registry.observe("OpenSubtitles", nil, os.ErrDeadlineExceeded)
	}
	if registry.before("OpenSubtitles") == nil {
		t.Fatal("repeated failures did not open the circuit")
	}
}

func TestSubtitleProviderHealthPersistsSafeStateAndBoundsRetryHeaders(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	registry := newSubtitleProviderHealthRegistry(data)
	registry.now = func() time.Time { return now }
	header := http.Header{}
	header.Set("Retry-After", "604801")
	header.Set("RateLimit-Remaining", "-1")
	header.Set("RateLimit-Reset", "300")
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Header: header}
	registry.observe("SubDL", response, nil)
	view := newSubtitleProviderHealthRegistry(data).views(map[string]bool{"SubDL": true})[0]
	if view.State != "limited" || view.Remaining != nil || view.ResetTime != now.Add(5*time.Minute).Format(time.RFC3339) || view.NextRetry != view.ResetTime || view.LastSafeError != "Rate limit reached" {
		t.Fatalf("restored provider state = %#v", view)
	}
	invalid := http.Header{}
	invalid.Set("RateLimit-Reset", "9223372036854775807")
	if _, reset := subtitleQuota(invalid, now, nil, time.Time{}); !reset.IsZero() {
		t.Fatalf("oversized reset was accepted: %v", reset)
	}
}

func TestSubtitleSearchBackoffAdaptsToRepeatedNoResults(t *testing.T) {
	t.Parallel()
	ledger := newSubtitleLedger("")
	key := subtitleSearchKey("0123456789abcdef", "en", "standard")
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	ledger.noteSearch(key, "no-result", "No trusted subtitle found", now)
	first, _, _ := ledger.search(key)
	ledger.noteSearch(key, "no-result", "No trusted subtitle found", now.Add(time.Hour))
	second, _, _ := ledger.search(key)
	if first.NextAt != now.Add(6*time.Hour).Unix() || second.NextAt != now.Add(time.Hour+24*time.Hour).Unix() || ledger.automaticSearchReady(key, now.Add(12*time.Hour)) {
		t.Fatalf("adaptive records = %#v, %#v", first, second)
	}
}

func TestSubtitleRecoveryRestoresPreviousFileAndFreezesReplacement(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.mp4")}
	target := subtitleSidecarPath(item, "en")
	current := []byte("1\n00:00:01,000 --> 00:00:02,000\nCurrent\n\n")
	previous := []byte("1\n00:00:01,000 --> 00:00:02,000\nPrevious\n\n")
	if err := os.WriteFile(target, current, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".kinosail.bak", previous, 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: "https://example.com", APIKey: "key"}, t.TempDir(), data, nil, nil, "")
	if err := provider.restorePrevious(item, "en"); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(target)
	backup, _ := os.ReadFile(target + ".kinosail.bak")
	record, found, err := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if !strings.Contains(string(restored), "Previous") || !strings.Contains(string(backup), "Current") || err != nil || !found || !record.Frozen || record.Managed {
		t.Fatalf("restored = %q, backup = %q, record = %#v, %v", restored, backup, record, err)
	}
}

func TestSubtitlePlanPrefersSDHWithoutRenamingExistingFiles(t *testing.T) {
	t.Parallel()
	settings := newSettingsStore(t.TempDir(), "", "", nil)
	if err := settings.setSubtitlePlan("en", "sdh"); err != nil {
		t.Fatal(err)
	}
	manager := &subtitleManager{settings: settings}
	item := library.Item{Kind: "video", Path: "/media/Arrival.mp4", Subtitles: []string{"/media/Arrival.en.srt"}}
	if ready, _ := manager.planCoverage(t.Context(), item, "en"); ready {
		t.Fatal("standard subtitle satisfied an SDH preference")
	}
	item.Subtitles = append(item.Subtitles, "/media/Arrival.en.sdh.srt")
	if ready, _ := manager.planCoverage(t.Context(), item, "en"); !ready {
		t.Fatal("existing SDH subtitle was not retained and selected")
	}
}

func TestPiecewiseSubtitleAlignmentRequiresRegionalImprovement(t *testing.T) {
	t.Parallel()
	cues := make([]subtitleCue, 0, 90)
	position := 10
	for index := 0; index < 90; index++ {
		position += 12 + (index*7)%17
		start := time.Duration(position) * time.Second
		cues = append(cues, subtitleCue{Start: start, End: start + 4*time.Second, Text: "Dialogue"})
	}
	audio := make([]float64, int(31*time.Minute/subtitleFrame))
	offsets := []time.Duration{-2 * time.Second, 0, 2 * time.Second}
	for index, cue := range cues {
		offset := offsets[min(index/30, 2)]
		start := int((cue.Start + offset) / subtitleFrame)
		end := int((cue.End + offset) / subtitleFrame)
		for frame := max(0, start); frame < min(len(audio), end); frame++ {
			audio[frame] = 1
		}
	}
	anchors, ok := findPiecewiseSubtitleAlignment(audio, cues, subtitleAlignment{Scale: 1, Offset: 0, Score: 0.2, Margin: 0.1})
	if !ok || len(anchors) != 3 || anchors[0].Offset >= anchors[2].Offset {
		t.Fatalf("piecewise anchors = %#v, %v", anchors, ok)
	}
}
