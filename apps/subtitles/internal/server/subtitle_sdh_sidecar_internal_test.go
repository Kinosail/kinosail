package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type sdhFixtureCandidate struct {
	text string
	hi   bool
}

func sdhFixture(t *testing.T, preference string, candidates ...sdhFixtureCandidate) (*subtitleProvider, library.Item) {
	t.Helper()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/subtitles" {
			subtitles := make([]subDLSubtitle, 0, len(candidates))
			for index, candidate := range candidates {
				subtitles = append(subtitles, subDLSubtitle{ReleaseName: "Movie.2026.BluRay-GROUP", Language: "en", HI: candidate.hi, URL: "/candidate/" + string(rune('a'+index)) + ".srt"})
			}
			_ = json.NewEncoder(writer).Encode(subDLResponse{Status: true, Results: []subDLResult{{Type: "movie", Name: "Movie", Year: 2026}}, Subtitles: subtitles})
			return
		}
		for index, candidate := range candidates {
			if request.URL.Path == "/candidate/"+string(rune('a'+index))+".srt" {
				_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\n" + candidate.text + "\n"))
				return
			}
		}
		http.NotFound(writer, request)
	}))
	t.Cleanup(remote.Close)
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Title: "Movie", Year: "2026", Path: filepath.Join(t.TempDir(), "Movie.2026.BluRay-GROUP.mp4")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := &settingsStore{file: "settings.json", value: installationSettings{SubtitleLanguages: []string{"en"}, SubtitlePreference: preference}, persist: func(string, any) error { return nil }}
	return newSubtitleProvider(SubtitleConfig{URL: remote.URL + "/api/v1", APIKey: "key"}, t.TempDir(), t.TempDir(), sidecarTestIndex(item), settings, ""), item
}

func sdhPaths(item library.Item) (string, string) {
	base := strings.TrimSuffix(item.Path, filepath.Ext(item.Path))
	return base + ".en.srt", base + ".en.sdh.srt"
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSDHPreferenceSavesCaptionsAsAnSDHFile(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "sdh", sdhFixtureCandidate{"[door creaks] Captions", true})
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	standard, sdh := sdhPaths(item)
	if !strings.Contains(fileText(t, sdh), "Captions") || fileText(t, standard) != "" {
		t.Fatalf("standard=%q sdh=%q", fileText(t, standard), fileText(t, sdh))
	}
	record, found, err := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if err != nil || !found || record.Variant != "sdh" || record.Role != "captions" {
		t.Fatalf("record = %#v, %t, %v", record, found, err)
	}
	if got := provider.managedSidecarPath(item, "en"); got != sdh {
		t.Fatalf("managed path = %q, want %q", got, sdh)
	}
}

func TestSDHPreferenceAddsCaptionsBesideAHouseholdFile(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "sdh", sdhFixtureCandidate{"Standard", false}, sdhFixtureCandidate{"[music] Captions", true})
	standard, sdh := sdhPaths(item)
	if err := os.WriteFile(standard, []byte("1\n00:00:01,000 --> 00:00:02,000\nHousehold\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fileText(t, standard), "Household") || !strings.Contains(fileText(t, sdh), "Captions") {
		t.Fatalf("standard=%q sdh=%q", fileText(t, standard), fileText(t, sdh))
	}
}

func TestSDHPreferenceDoesNotAddASecondStandardFile(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "sdh", sdhFixtureCandidate{"Standard", false})
	standard, sdh := sdhPaths(item)
	if err := os.WriteFile(standard, []byte("1\n00:00:01,000 --> 00:00:02,000\nHousehold\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provider.fetchSidecar(t.Context(), item, "en"); !errors.Is(err, errNoTrustedSubtitle) {
		t.Fatalf("fetch = %v, want no trusted subtitle", err)
	}
	if fileText(t, sdh) != "" || !strings.Contains(fileText(t, standard), "Household") {
		t.Fatalf("standard=%q sdh=%q", fileText(t, standard), fileText(t, sdh))
	}
}

func TestSDHPreferenceFallsBackToAStandardFileWhenNoCaptionsExist(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "sdh", sdhFixtureCandidate{"Standard", false})
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	standard, sdh := sdhPaths(item)
	if !strings.Contains(fileText(t, standard), "Standard") || fileText(t, sdh) != "" {
		t.Fatalf("standard=%q sdh=%q", fileText(t, standard), fileText(t, sdh))
	}
	record, _, _ := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if record.Variant != "" || provider.managedSidecarPath(item, "en") != standard {
		t.Fatalf("fallback record = %#v", record)
	}
}

func TestStandardPreferenceKeepsTheStandardFileName(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "standard", sdhFixtureCandidate{"[music] Captions", true})
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	standard, sdh := sdhPaths(item)
	if !strings.Contains(fileText(t, standard), "Captions") || fileText(t, sdh) != "" {
		t.Fatalf("standard=%q sdh=%q", fileText(t, standard), fileText(t, sdh))
	}
}

func TestReviewAndUpgradeUseTheManagedSDHFile(t *testing.T) {
	t.Parallel()
	provider, item := sdhFixture(t, "sdh", sdhFixtureCandidate{"[music] Captions", true})
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	standard, sdh := sdhPaths(item)
	if err := os.WriteFile(standard, []byte("1\n00:00:01,000 --> 00:00:02,000\nHousehold\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	item.Subtitles = []string{standard, sdh}
	path, data, err := provider.reviewSource(item, "en")
	if err != nil || path != sdh || !strings.Contains(string(data), "Captions") {
		t.Fatalf("review source = %q %q %v", path, data, err)
	}
	target, err := provider.openSidecar(item, "en")
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	if target.name != filepath.Base(sdh) {
		t.Fatalf("managed sidecar = %q", target.name)
	}
	record, _, _ := provider.ledger.record(subtitleRecordKey(item.ID, "en"))
	if subtitleUpgradeImproves(subtitleDownloadCandidate{Score: record.Score + 50}, record) || !subtitleUpgradeImproves(subtitleDownloadCandidate{Score: record.Score + 50, HI: true}, record) {
		t.Fatal("an SDH file can be upgraded to a standard subtitle")
	}
}
