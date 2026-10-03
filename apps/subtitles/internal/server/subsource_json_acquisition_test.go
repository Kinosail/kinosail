package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubSourceRejectsAmbiguousJSONBeforeAcquisition(t *testing.T) { //nolint:cyclop,funlen,gocognit // One real acquisition matrix checks ambiguity rejection and durable no-write controls.
	t.Parallel()
	for _, fixture := range []struct {
		name, fields string
		allowed      bool
	}{
		{"machine-only", `"productionType":"machine"`, false},
		{"retail-only", `"productionType":"retail"`, true},
		{"retail-unknown-metadata", `"productionType":"retail","communityMetadata":{"source":"fixture","count":3}`, true},
		{"duplicate-canonical", `"productionType":"machine","productionType":"retail"`, false},
		{"case-alias", `"productionType":"machine","ProductionType":"retail"`, false},
		{"escaped-identical-key", `"productionType":"machine","production\u0054ype":"retail"`, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			original := []byte("1\n00:00:01,000 --> 00:00:02,000\nIsolated provider dialogue\n")
			archive := zipSubSourceFiles(t, map[string][]byte{"Show.S01E02.srt": original})
			response := fmt.Sprintf(`{"success":true,"data":[{"subtitleId":9,"movieId":7,"language":"english","releaseInfo":["Show.S01E02.WEB-DL"],"files":1,"size":%d,%s,"downloads":10,"rating":{"good":4,"bad":0,"total":4}}],"pagination":{"page":1,"limit":50,"total":1,"pages":1}}`, len(archive), fixture.fields)
			if !json.Valid([]byte(response)) {
				t.Fatal("fixture is not valid JSON")
			}
			var searches, lists, downloads atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-API-Key") != "isolated-fixture-key" {
					http.Error(w, "fixture credential missing", http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/api/v1/movies/search":
					searches.Add(1)
					if r.URL.Query().Get("imdb") != "tt1234567" {
						http.Error(w, "fixture identity mismatch", http.StatusBadRequest)
						return
					}
					_, _ = w.Write([]byte(`{"success":true,"data":[{"movieId":7,"title":"Show","type":"tvseries","releaseYear":2024,"imdbId":"tt1234567","season":1,"subtitleCount":1}]}`))
				case "/api/v1/subtitles":
					lists.Add(1)
					_, _ = w.Write([]byte(response))
				case "/api/v1/subtitles/9/download":
					downloads.Add(1)
					w.Header().Set("Content-Type", "application/zip")
					_, _ = w.Write(archive)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(remote.Close)
			media := filepath.Join(t.TempDir(), "Show.S01E02.mkv")
			if err := os.WriteFile(media, []byte("isolated media locator"), 0o600); err != nil {
				t.Fatal(err)
			}
			item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: media, Title: "Second", Show: "Show", ShowYear: "2024", Season: 1, Episode: 2, ShowProviderIDs: map[string]string{"imdb": "tt1234567"}}
			data := t.TempDir()
			provider := newSubtitleProvider(SubtitleConfig{SubSource: SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "isolated-fixture-key", PersonalUse: true}}, t.TempDir(), data, sidecarTestIndex(item), nil, "")
			fetchErr := provider.fetchSidecar(t.Context(), item, "en")
			installed, readErr := os.ReadFile(filepath.Join(filepath.Dir(media), "Show.S01E02.en.srt"))
			ledgerData, err := os.ReadFile(filepath.Join(data, "subtitle_acquisitions.json"))
			if err != nil {
				t.Fatal(err)
			}
			var ledger struct {
				Records map[string]json.RawMessage
				History []json.RawMessage
			}
			if err := json.Unmarshal(ledgerData, &ledger); err != nil {
				t.Fatal(err)
			}
			originals, originalErr := os.ReadDir(filepath.Join(data, "subtitle-originals"))
			if originalErr != nil && !os.IsNotExist(originalErr) {
				t.Fatal(originalErr)
			}
			if searches.Load() != 1 || lists.Load() != 1 {
				t.Fatalf("candidate fixture was not reached: searches=%d lists=%d", searches.Load(), lists.Load())
			}
			if fixture.allowed {
				if fetchErr != nil || readErr != nil || !bytes.Equal(installed, original) || downloads.Load() != 1 || len(ledger.Records) != 1 || len(ledger.History) != 1 || len(originals) != 1 {
					t.Errorf("valid candidate did not install with exact bytes and acquisition provenance")
				}
			} else if fetchErr == nil || !os.IsNotExist(readErr) || downloads.Load() != 0 || len(ledger.Records) != 0 || len(ledger.History) != 0 || len(originals) != 0 {
				t.Errorf("forbidden or ambiguous provider JSON caused acquisition effects: downloads=%d installed=%t records=%d history=%d originals=%d", downloads.Load(), readErr == nil, len(ledger.Records), len(ledger.History), len(originals))
			}
		})
	}
}
