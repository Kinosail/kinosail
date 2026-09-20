package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleCredentialCheckExercisesEveryConfiguredProvider(t *testing.T) {
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v2/me", "/api/v1/languages":
			_, _ = writer.Write([]byte(`{}`))
		case "/api/v1/login":
			_, _ = writer.Write([]byte(`{"token":"token","status":200}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(remote.Close)
	config := SubtitleConfig{
		URL:           remote.URL + "/api/v1",
		APIKey:        "subdl-key",
		OpenSubtitles: OpenSubtitlesConfig{URL: remote.URL + "/api/v1", APIKey: "open-key", Username: "owner", Password: "password"},
		SubSource:     SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "source-key", PersonalUse: true},
	}
	provider := newSubtitleProvider(config, t.TempDir(), t.TempDir(), nil, nil, "")
	if attempted, connected := provider.testCredentials(t.Context()); attempted != 3 || connected != 3 {
		t.Fatalf("credential result = %d attempted, %d connected", attempted, connected)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)
	config.URL, config.OpenSubtitles.URL, config.SubSource.URL = failing.URL+"/api/v1", failing.URL+"/api/v1", failing.URL+"/api/v1"
	provider = newSubtitleProvider(config, t.TempDir(), t.TempDir(), nil, nil, "")
	if attempted, connected := provider.testCredentials(t.Context()); attempted != 3 || connected != 0 {
		t.Fatalf("failed credential result = %d attempted, %d connected", attempted, connected)
	}
}

func TestSubDLSearchUsesSeriesIdentity(t *testing.T) {
	t.Parallel()
	query := make(chan string, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query <- request.URL.RawQuery
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":false}`))
	}))
	t.Cleanup(remote.Close)
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), t.TempDir(), nil, nil, "")
	item := library.Item{Path: "/media/Series.S02E03.mkv", Show: "Series", Season: 2, Episode: 3, ShowProviderIDs: map[string]string{"imdb": "tt1234567"}}
	if _, err := provider.searchSubDL(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(<-query)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("type") != "tv" || values.Get("film_name") != "Series" || values.Get("season_number") != "2" || values.Get("episode_number") != "3" || values.Get("imdb_id") != "tt1234567" {
		t.Fatalf("series query = %v", values)
	}
}

func TestSubtitleV1ValidationAndMigrationBranches(t *testing.T) { //nolint:cyclop // One table covers persisted v1 compatibility and strict rejection branches.
	t.Parallel()
	now := time.Now().Unix()
	record := subtitleRecord{Fingerprint: strings.Repeat("0", 64), Source: "external", CheckedAt: now, InstalledAt: now, Synchronization: "none"}
	search := subtitleSearchRecord{Outcome: "no-result", CheckedAt: now, NextAt: now + 1}
	state := subtitleLedgerState{
		Version:  1,
		Records:  map[string]subtitleRecord{"0123456789abcdef:en": record},
		Searches: map[string]subtitleSearchRecord{"0123456789abcdef:en:standard": search},
	}
	if !validSubtitleLedgerState(state) || !validSubtitleDay("2026-08-30") || validSubtitleDay("2026-8-30") {
		t.Fatal("valid v1 state or strict day validation failed")
	}
	for name, mutate := range map[string]func(subtitleLedgerState) subtitleLedgerState{
		"version": func(value subtitleLedgerState) subtitleLedgerState { value.Version = 0; return value },
		"search key": func(value subtitleLedgerState) subtitleLedgerState {
			value.Searches = map[string]subtitleSearchRecord{"bad": search}
			return value
		},
		"record key": func(value subtitleLedgerState) subtitleLedgerState {
			value.Records = map[string]subtitleRecord{"bad": record}
			return value
		},
		"day": func(value subtitleLedgerState) subtitleLedgerState { value.SubDLDay = "bad"; return value },
	} {
		t.Run(name, func(t *testing.T) {
			if validSubtitleLedgerState(mutate(state)) {
				t.Fatal("invalid ledger state was accepted")
			}
		})
	}

	data := t.TempDir()
	ledgerPath := filepath.Join(data, "subtitle_acquisitions.json")
	contents := `{"version":1,"records":{"0123456789abcdef:en":{"fingerprint":"` + strings.Repeat("0", 64) + `","source":"external","score":0,"checked_at":` + strconv.FormatInt(now, 10) + `,"managed":false,"installed_at":` + strconv.FormatInt(now, 10) + `,"synchronization":"none"}}}`
	if err := os.WriteFile(ledgerPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := newSubtitleLedger(data)
	if ledger.err != nil || ledger.state.Version != subtitleLedgerVersion || ledger.state.Searches == nil {
		t.Fatalf("v1 migration = %#v, %v", ledger.state, ledger.err)
	}

	broken := newSubtitleLedger("")
	broken.path = t.TempDir()
	if broken.store("0123456789abcdef:en", record) == nil || broken.storeSearch("0123456789abcdef:en:standard", search) == nil {
		t.Fatal("durable store failure was accepted")
	}
}

func TestSubtitleV1RecoveryAndAlignmentUtilityBranches(t *testing.T) { //nolint:cyclop // One test covers small recovery and alignment boundary helpers.
	t.Parallel()
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), t.TempDir(), nil, nil, "")
	invalid := library.Item{Kind: "audio", ID: "0123456789abcdef"}
	if provider.setReplacement(invalid, "en", false) == nil || provider.restorePrevious(invalid, "en") == nil {
		t.Fatal("non-video recovery request was accepted")
	}
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Path: filepath.Join(t.TempDir(), "Movie.mp4")}
	if provider.setReplacement(item, "en", false) == nil || provider.restorePrevious(item, "en") == nil {
		t.Fatal("missing sidecar recovery request was accepted")
	}
	target := subtitleSidecarPath(item, "en")
	if err := os.WriteFile(target, []byte("subtitle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if provider.restorePrevious(item, "en") == nil {
		t.Fatal("missing recovery copy was accepted")
	}

	anchors := []subtitleAlignmentAnchor{{At: time.Second, Offset: -time.Second}, {At: 3 * time.Second, Offset: time.Second}}
	if subtitleInterpolatedOffset(0, nil) != 0 || subtitleInterpolatedOffset(time.Second, anchors) != -time.Second || subtitleInterpolatedOffset(2*time.Second, anchors) != 0 || subtitleInterpolatedOffset(4*time.Second, anchors) != time.Second {
		t.Fatal("piecewise interpolation returned an unexpected offset")
	}
	if subtitlePreferenceBonus(true, true) != 10 || subtitlePreferenceBonus(true, false) != 0 {
		t.Fatal("subtitle role preference bonus is invalid")
	}
}

func TestSubtitleLedgerRejectsCorruptionAndBoundsDownloads(t *testing.T) { //nolint:cyclop,funlen,gocognit // One test proves corrupt persistence and quota rollback branches.
	t.Parallel()
	for name, contents := range map[string]string{
		"malformed":  `{`,
		"unknown":    `{"version":2,"records":{},"unknown":true}`,
		"bad record": `{"version":2,"records":{"bad":{"fingerprint":"` + strings.Repeat("0", 64) + `","source":"external"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			data := t.TempDir()
			if err := os.WriteFile(filepath.Join(data, "subtitle_acquisitions.json"), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if ledger := newSubtitleLedger(data); ledger.err == nil {
				t.Fatal("corrupt ledger was accepted")
			}
		})
	}
	nilMaps := t.TempDir()
	if err := os.WriteFile(filepath.Join(nilMaps, "subtitle_acquisitions.json"), []byte(`{"version":2,"records":null,"searches":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded := newSubtitleLedger(nilMaps); loaded.err != nil || loaded.state.Records == nil || loaded.state.Searches == nil {
		t.Fatalf("nil ledger maps were not initialized: %#v, %v", loaded.state, loaded.err)
	}
	directoryData := t.TempDir()
	if err := os.Mkdir(filepath.Join(directoryData, "subtitle_acquisitions.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if ledger := newSubtitleLedger(directoryData); ledger.err == nil {
		t.Fatal("ledger directory was accepted as a file")
	}

	now := time.Now().UTC()
	ledger := newSubtitleLedger("")
	ledger.err = errors.New("unavailable")
	if ledger.takeSubDLDownload(now) {
		t.Fatal("download was reserved with an invalid ledger")
	}
	ledger.err = nil
	ledger.state.SubDLDay, ledger.state.SubDLDownloads = now.Format("2006-01-02"), subDLAutomaticDownloadLimit
	if ledger.takeSubDLDownload(now) {
		t.Fatal("daily download limit was exceeded")
	}
	ledger.state.SubDLDay = now.Add(-24 * time.Hour).Format("2006-01-02")
	if !ledger.takeSubDLDownload(now) || ledger.state.SubDLDownloads != 1 {
		t.Fatal("new UTC day did not reset the download limit")
	}
	ledger.path, ledger.state.SubDLDay, ledger.state.SubDLDownloads = t.TempDir(), "previous", 7
	if ledger.takeSubDLDownload(now) || ledger.state.SubDLDay != "previous" || ledger.state.SubDLDownloads != 7 {
		t.Fatal("failed quota persistence did not roll state back")
	}
	key := "0123456789abcdef:en"
	previousRecord := subtitleRecord{Fingerprint: strings.Repeat("0", 64), Source: "external", Score: 1}
	ledger.state.Records[key] = previousRecord
	if ledger.store(key, subtitleRecord{Fingerprint: strings.Repeat("1", 64), Source: "external", Score: 2}) == nil || ledger.state.Records[key].Fingerprint != previousRecord.Fingerprint || ledger.state.Records[key].Score != previousRecord.Score {
		t.Fatal("failed record update did not restore the previous record")
	}
	searchKey := "0123456789abcdef:en:standard"
	previousSearch := subtitleSearchRecord{Outcome: "no-result", CheckedAt: now.Unix(), NextAt: now.Add(time.Hour).Unix()}
	ledger.state.Searches[searchKey] = previousSearch
	nextSearch := previousSearch
	nextSearch.Attempts = 2
	if ledger.storeSearch(searchKey, nextSearch) == nil || ledger.state.Searches[searchKey] != previousSearch {
		t.Fatal("failed search update did not restore the previous record")
	}

	record := subtitleRecord{Fingerprint: strings.Repeat("0", 64), Source: "external"}
	if !validSubtitleRecord(record) || validSubtitleRecord(subtitleRecord{Fingerprint: strings.Repeat("F", 64), Source: "external"}) || validSubtitleRecord(subtitleRecord{Fingerprint: strings.Repeat("0", 64), Source: "unknown"}) {
		t.Fatal("subtitle record validation accepted an unsafe record")
	}
}

func TestSubtitleAcquireAndUpgradeFailureBranches(t *testing.T) { //nolint:cyclop // One provider fixture proves aggregate outage and managed upgrade rejection.
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(remote.Close)
	config := SubtitleConfig{
		URL:           remote.URL + "/api/v1",
		APIKey:        "subdl-key",
		OpenSubtitles: OpenSubtitlesConfig{URL: remote.URL + "/api/v1", APIKey: "open-key", Username: "owner", Password: "password"},
		SubSource:     SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "source-key", PersonalUse: true},
	}
	provider := newSubtitleProvider(config, t.TempDir(), t.TempDir(), nil, nil, "")
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Title: "Movie", Path: filepath.Join(t.TempDir(), "Movie.mp4")}
	if _, _, err := provider.acquire(t.Context(), item, "en", nil); !errors.Is(err, errSubtitleProviderUnavailable) {
		t.Fatalf("aggregate provider outage = %v", err)
	}
	if _, _, err := newSubtitleProvider(SubtitleConfig{}, "", "", nil, nil, "").acquire(t.Context(), item, "en", nil); err == nil {
		t.Fatal("unconfigured acquisition was accepted")
	}
	if provider.upgradeEligible(library.Item{Kind: "audio"}, "en", time.Now()) || provider.upgradeEligible(item, "en", time.Now()) {
		t.Fatal("unsafe upgrade target was eligible")
	}
	target := subtitleSidecarPath(item, "en")
	if err := os.WriteFile(target, []byte("subtitle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !provider.upgradeEligible(item, "en", time.Now()) {
		t.Fatal("external sidecar was not eligible for a trusted upgrade")
	}
	if upgraded, err := provider.upgradeSidecar(t.Context(), library.Item{Kind: "audio"}, "en"); upgraded || err == nil {
		t.Fatal("invalid upgrade request was accepted")
	}
	if data, err := readUpgradeSidecar(target); err != nil || string(data) != "subtitle" {
		t.Fatalf("valid upgrade sidecar = %q, %v", data, err)
	}
	if _, err := readUpgradeSidecar(t.TempDir()); err == nil {
		t.Fatal("sidecar directory was accepted")
	}
	empty := filepath.Join(t.TempDir(), "empty.srt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readUpgradeSidecar(empty); err == nil {
		t.Fatal("empty sidecar was accepted")
	}
	if subtitleTrackListed([]string{"Embedded en"}, "Missing") {
		t.Fatal("missing subtitle track was reported as present")
	}
}

func TestReadUpgradeSidecarRejectsReplacementAfterPathCheck(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "movie.en.srt")
	replacement := filepath.Join(directory, "replacement.txt")
	original := filepath.Join(directory, "original.srt")
	if err := os.WriteFile(path, []byte("subtitle"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readUpgradeSidecarWith(path, os.Lstat, func(name string) (*os.File, error) {
		if err := os.Rename(name, original); err != nil {
			return nil, err
		}
		if err := os.Symlink(replacement, name); err != nil {
			return nil, err
		}
		return os.Open(name)
	}); err == nil {
		t.Fatal("sidecar replacement race was accepted")
	}
}
