package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubSourceAcquisitionRequiresExactEpisode(t *testing.T) { //nolint:cyclop,funlen,gocognit // One acquisition matrix checks both provider boundaries and durable installation effects.
	t.Parallel()
	for _, episode := range []struct {
		name, token string
		valid       bool
	}{
		{"padded", "S01E02", true},
		{"unpadded", "S1E2", true},
		{"attached show title", "ShowS01E02", true},
		{"mixed case", "s01e02", true},
		{"repeated equal identity", "S01E02.S01E02", true},
		{"quality suffix", "S01E02-1080p", true},
		{"year suffix", "S01E02.2024", true},
		{"numeric range", "S01E02-03", false},
		{"spaced numeric range", "S01E02 - 03", false},
		{"combining mark suffix", "S01E02\u0301", false},
		{"hyphen episode continuation", "S01E02-E03", false},
		{"plus episode continuation", "S01E02+E03", false},
		{"spaced episode continuation", "S01E02 - E03", false},
		{"fullwidth digit suffix", "S01E02０", false},
		{"Arabic digit suffix", "S01E02٣", false},
		{"Unicode letter suffix", "S01E02é", false},
		{"Unicode marker alias", "ſ01E02", false},
		{"longer unpadded number", "S1E20", false},
		{"longer padded number", "S01E020", false},
		{"three digit number", "S1E200", false},
		{"wrong season", "S11E02", false},
		{"combined episodes", "S01E02E03", false},
		{"conflicting episodes", "S01E02.S01E03", false},
		{"numeric overflow", "S1E99999999999999999999999999999", false},
		{"missing episode", "Season.1", false},
	} {
		for _, mode := range []string{"single release", "season archive", "valid release with one member"} {
			t.Run(episode.name+"/"+mode, func(t *testing.T) {
				original := []byte("1\n00:00:01,000 --> 00:00:02,000\nDialogue from " + episode.token + "\n")
				member := "Show." + episode.token + ".srt"
				release, files := "Show."+episode.token+".WEB-DL", 1
				archiveFiles := map[string][]byte{member: original}
				if mode == "season archive" {
					release, files = "Show.Season.1.WEB-DL", 2
					archiveFiles["Show.S01E03.srt"] = []byte("1\n00:00:01,000 --> 00:00:02,000\nThird episode\n")
				}
				if mode == "valid release with one member" {
					release = "Show.S01E02.WEB-DL"
				}
				archive := zipSubSourceFiles(t, archiveFiles)
				var downloads atomic.Int32
				remote := newSubSourceEpisodeServer(t, release, files, archive, &downloads)
				media := filepath.Join(t.TempDir(), "Show.S01E02.mkv")
				if err := os.WriteFile(media, []byte("isolated media locator"), 0o600); err != nil {
					t.Fatal(err)
				}
				item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: media, Title: "Second", Show: "Show", ShowYear: "2024", Season: 1, Episode: 2, ShowProviderIDs: map[string]string{"imdb": "tt1234567"}}
				data := t.TempDir()
				config := SubtitleConfig{SubSource: SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "key", PersonalUse: true}}
				provider := newSubtitleProvider(config, t.TempDir(), data, sidecarTestIndex(item), nil, "")
				err := provider.fetchSidecar(t.Context(), item, "en")
				installed, readErr := os.ReadFile(filepath.Join(filepath.Dir(media), "Show.S01E02.en.srt"))
				if episode.valid {
					if err != nil || readErr != nil || !bytes.Equal(installed, original) || downloads.Load() != 1 {
						t.Fatalf("exact episode acquisition failed: fetch=%v read=%v downloads=%d data=%q", err, readErr, downloads.Load(), installed)
					}
				} else {
					if err == nil || !os.IsNotExist(readErr) {
						t.Fatalf("wrong episode installed: fetch=%v read=%v data=%q", err, readErr, installed)
					}
					expectedDownloads := int32(0)
					if mode != "single release" {
						expectedDownloads = 1
					}
					if downloads.Load() != expectedDownloads {
						t.Fatalf("unexpected download count: got=%d want=%d", downloads.Load(), expectedDownloads)
					}
				}
				ledgerData, ledgerErr := os.ReadFile(filepath.Join(data, "subtitle_acquisitions.json"))
				if ledgerErr != nil {
					t.Fatal(ledgerErr)
				}
				var ledger struct {
					Records map[string]json.RawMessage
					History []json.RawMessage
				}
				if err := json.Unmarshal(ledgerData, &ledger); err != nil {
					t.Fatal(err)
				}
				originals, originalErr := os.ReadDir(filepath.Join(data, "subtitle-originals"))
				expected := 0
				if episode.valid {
					expected = 1
				}
				if originalErr != nil && !os.IsNotExist(originalErr) {
					t.Fatal(originalErr)
				}
				if len(ledger.Records) != expected || len(ledger.History) != expected || len(originals) != expected {
					t.Fatalf("acquisition effects: records=%d history=%d originals=%d want=%d", len(ledger.Records), len(ledger.History), len(originals), expected)
				}
			})
		}
	}
}

func newSubSourceEpisodeServer(t *testing.T, release string, files int, archive []byte, downloads *atomic.Int32) *httptest.Server {
	t.Helper()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-API-Key") != "key" {
			http.Error(writer, "missing fixture credential", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v1/movies/search":
			if request.URL.Query().Get("imdb") != "tt1234567" {
				http.Error(writer, "wrong fixture query", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":[{"movieId":7,"title":"Show","type":"tvseries","releaseYear":2024,"imdbId":"tt1234567","season":1,"subtitleCount":1}]}`))
		case "/api/v1/subtitles":
			_ = json.NewEncoder(writer).Encode(map[string]any{"success": true, "data": []any{map[string]any{"subtitleId": 9, "movieId": 7, "language": "english", "releaseInfo": []string{release}, "files": files, "size": len(archive), "productionType": "retail", "downloads": 10, "rating": map[string]int{"good": 4, "bad": 0, "total": 4}}}, "pagination": map[string]int{"page": 1, "limit": 50, "total": 1, "pages": 1}})
		case "/api/v1/subtitles/9/download":
			downloads.Add(1)
			writer.Header().Set("Content-Type", "application/zip")
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(remote.Close)
	return remote
}
