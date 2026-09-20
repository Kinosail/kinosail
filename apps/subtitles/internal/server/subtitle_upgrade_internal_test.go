package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestSubtitleProviderUpgradesManagedSidecarByMinimumScoreGain(t *testing.T) { //nolint:cyclop,gocognit // One test proves selection, backup, rollback, freeze, and ledger state.
	t.Parallel()
	var better atomic.Bool
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/subtitle.srt" {
			text := "Original"
			if better.Load() {
				text = "Better"
			}
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\n" + text + "\n"))
			return
		}
		productionType := 3
		if better.Load() {
			productionType = 0
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie", "imdb_id": "tt123"}},
			"subtitles": []any{map[string]any{"url": remoteURL(request, "/subtitle.srt"), "language": "EN", "release_name": "Arrival.BluRay-GROUP", "production_type": productionType}},
		})
	}))
	t.Cleanup(remote.Close)
	media, data := t.TempDir(), t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), ProviderIDs: map[string]string{"imdb": "tt123"}}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), data, sidecarTestIndex(item), nil, "")
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	key := subtitleRecordKey(item.ID, "en")
	record, found, err := provider.ledger.record(key)
	if err != nil || !found || record.Score != 70 || !record.Managed {
		t.Fatalf("initial record = %#v, found = %v, err = %v", record, found, err)
	}
	record.CheckedAt = time.Now().Add(-48 * time.Hour).Unix()
	if err = provider.ledger.store(key, record); err != nil {
		t.Fatal(err)
	}
	better.Store(true)
	upgraded, err := provider.upgradeSidecar(t.Context(), item, "en")
	current, readErr := os.ReadFile(subtitleSidecarPath(item, "en"))
	if err != nil || readErr != nil || !upgraded || !strings.Contains(string(current), "Better") {
		t.Fatalf("upgrade = %v, err = %v, sidecar = %q, read = %v", upgraded, err, current, readErr)
	}
	backup, backupErr := os.ReadFile(subtitleSidecarPath(item, "en") + ".kinosail.bak")
	if backupErr != nil || !strings.Contains(string(backup), "Original") {
		t.Fatalf("managed upgrade backup = %q, %v", backup, backupErr)
	}
	item.Subtitles = []string{subtitleSidecarPath(item, "en")}
	record, _, err = provider.ledger.record(key)
	record.CheckedAt = time.Now().Add(-48 * time.Hour).Unix()
	if err != nil || provider.ledger.store(key, record) != nil {
		t.Fatalf("age upgraded record: %v", err)
	}
	result := (&subtitleManager{provider: provider}).maintainItems(t.Context(), []library.Item{item}, "en", 1, 0)
	if result.Attempted != 1 || result.Upgraded != 0 || result.Failed != 0 {
		t.Fatalf("no-better maintenance = %#v", result)
	}
}

func TestSubtitleMaintenanceAddsEveryPreferredLanguage(t *testing.T) {
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/subtitles" {
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nFound\n"))
			return
		}
		language := request.URL.Query().Get("languages")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie", "imdb_id": "tt123"}},
			"subtitles": []any{map[string]any{"url": remoteURL(request, "/"+strings.ToLower(language)+".srt"), "language": language, "release_name": "Arrival.BluRay-GROUP"}},
		})
	}))
	t.Cleanup(remote.Close)
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), ProviderIDs: map[string]string{"imdb": "tt123"}}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), t.TempDir(), sidecarTestIndex(item), nil, "")
	result := (&subtitleManager{provider: provider}).maintainLanguageItems(t.Context(), []library.Item{item}, []string{"eng", "spa"}, 2, 0)
	if result.Attempted != 2 || result.Added != 2 || result.Failed != 0 {
		t.Fatalf("maintenance = %#v", result)
	}
	for _, language := range []string{"en", "es"} {
		if _, err := os.Stat(subtitleSidecarPath(item, language)); err != nil {
			t.Errorf("%s sidecar: %v", language, err)
		}
	}
}

func TestSubtitleAutomationCursorContinuesAfterFailedLanguage(t *testing.T) { //nolint:cyclop // The integration flow checks retry cursor semantics.
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v2/me" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"status": true})
			return
		}
		if request.URL.Path != "/subtitles" {
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nFound\n"))
			return
		}
		language := request.URL.Query().Get("languages")
		subtitles := []any{}
		if language == "ES" {
			subtitles = []any{map[string]any{"url": remoteURL(request, "/es.srt"), "language": "ES", "release_name": "Arrival.BluRay-GROUP"}}
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie", "imdb_id": "tt123"}}, "subtitles": subtitles,
		})
	}))
	t.Cleanup(remote.Close)
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), ProviderIDs: map[string]string{"imdb": "tt123"}}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := sidecarTestIndex(item)
	settings := newSettingsStore(media, t.TempDir(), "", nil)
	settings.value.SubtitleLanguage, settings.value.SubtitleLanguages = "en", []string{"en", "es"}
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), t.TempDir(), index, settings, "")
	if attempted, connected := provider.testCredentials(t.Context()); attempted != 1 || connected != 1 {
		t.Fatalf("provider health = %d, %d", attempted, connected)
	}
	backups := newBackupManager(context.Background(), t.TempDir(), t.TempDir(), "test-backup-key", 0, 7, workload.New(1))
	manager := newSubtitleManager(index, settings, provider, nil, backups)
	first, second := manager.automate(t.Context(), 1), manager.automate(t.Context(), 1)
	if first.Attempted != 1 || first.Failed != 1 || second.Attempted != 1 || second.Added != 1 {
		t.Fatalf("maintenance did not advance languages: first=%+v second=%+v", first, second)
	}
	if _, err := os.Stat(subtitleSidecarPath(item, "es")); err != nil {
		t.Fatalf("Spanish sidecar: %v", err)
	}
}

func TestSubtitleProviderOnlyReplacesUnknownSidecarWithExactHashMatch(t *testing.T) { //nolint:cyclop // One fake official API proves login, exact-match replacement, backup, and provenance.
	t.Parallel()
	var remote *httptest.Server
	remote = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"total_count": 1,
				"data": []any{map[string]any{
					"id": "one", "type": "subtitle",
					"attributes": map[string]any{
						"language": "en", "moviehash_match": true, "from_trusted": true, "release": "Arrival", "nb_cd": 1,
						"feature_details": map[string]any{"feature_type": "movie", "title": "Arrival"},
						"files":           []any{map[string]any{"file_id": 7, "cd_number": 1, "file_name": "Arrival.srt"}},
					},
				}},
			})
		case "/login":
			_ = json.NewEncoder(writer).Encode(map[string]any{"token": "session", "status": 200})
		case "/download":
			_ = json.NewEncoder(writer).Encode(map[string]any{"link": remote.URL + "/file.srt"})
		case "/file.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nExact\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(remote.Close)
	media := t.TempDir()
	item := library.Item{ID: "fedcba9876543210", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.mp4")}
	if err := os.WriteFile(item.Path, make([]byte, 140000), 0o600); err != nil {
		t.Fatal(err)
	}
	target := subtitleSidecarPath(item, "en")
	if err := os.WriteFile(target, []byte("1\n00:00:01,000 --> 00:00:02,000\nUnknown\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := SubtitleConfig{OpenSubtitles: OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "user", Password: "password"}}
	provider := newSubtitleProvider(config, t.TempDir(), t.TempDir(), sidecarTestIndex(item), nil, "")
	upgraded, err := provider.upgradeSidecar(t.Context(), item, "en")
	current, currentErr := os.ReadFile(target)
	backup, backupErr := os.ReadFile(target + ".kinosail.bak")
	if err != nil || currentErr != nil || backupErr != nil || !upgraded || !strings.Contains(string(current), "Exact") || !strings.Contains(string(backup), "Unknown") {
		t.Fatalf("upgrade = %v, err = %v, current = %q/%v, backup = %q/%v", upgraded, err, current, currentErr, backup, backupErr)
	}
}

func TestSubtitleLedgerRejectsMalformedPersistedStateAndStopsDownloads(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	path := filepath.Join(data, "subtitle_acquisitions.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"records":{"bad":{"fingerprint":"x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := newSubtitleLedger(data)
	if ledger.err == nil || ledger.takeSubDLDownload(time.Now()) {
		t.Fatalf("invalid ledger = %v, download allowed", ledger.err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("invalid state changed: %v, %#v", err, info)
	}
}

func remoteURL(request *http.Request, path string) string {
	return "http://" + request.Host + path
}
