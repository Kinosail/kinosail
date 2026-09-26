package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleMaintenanceAPIUpgradesExactUnknownSidecarAndKeepsBackup(t *testing.T) { //nolint:cyclop // The versioned adapter proves exact-hash adoption, replacement, response, and recovery backup.
	t.Parallel()
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			writeExactOpenSubtitlesSearch(writer)
		case "/login":
			_ = json.NewEncoder(writer).Encode(map[string]any{"token": "session", "status": 200})
		case "/download":
			_ = json.NewEncoder(writer).Encode(map[string]any{"link": provider.URL + "/file.srt"})
		case "/file.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nExact\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), strings.Repeat("v", 140000))
	writeTestFile(t, filepath.Join(media, "Arrival.en.srt"), "1\n00:00:01,000 --> 00:00:02,000\nUnknown\n")
	config := server.SubtitleConfig{OpenSubtitles: server.OpenSubtitlesConfig{URL: provider.URL, APIKey: "key", Username: "user", Password: "password"}}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: config})
	response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/maintain", `{"limit":1}`)
	current, currentErr := os.ReadFile(filepath.Join(media, "Arrival.en.srt"))
	backup, backupErr := os.ReadFile(filepath.Join(media, "Arrival.en.srt.kinosail.bak"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"upgraded":1`) || currentErr != nil || backupErr != nil || !strings.Contains(string(current), "Exact") || !strings.Contains(string(backup), "Unknown") {
		t.Fatalf("maintain = %d %q, current = %q/%v, backup = %q/%v", response.Code, response.Body.String(), current, currentErr, backup, backupErr)
	}
}

func TestSubtitleMaintenanceWebAddsMissingSidecar(t *testing.T) {
	t.Parallel()
	provider := newMultilingualSubDL(t)
	media := t.TempDir()
	data := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL}})
	if saved := requestJSON(t, handler, http.MethodPut, "/api/v1/configuration/integrations.subdl.api_key", `{"value":"key"}`); saved.Code != http.StatusAccepted || !strings.Contains(saved.Body.String(), `"restartRequired":false`) {
		t.Fatalf("provider save = %d %q", saved.Code, saved.Body.String())
	}
	if settings := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es"]}`); settings.Code != http.StatusOK {
		t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
	}
	response := requestApp(t, handler, http.MethodPost, "/subtitles/manage/maintain", "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("maintain = %d %q", response.Code, response.Header().Get("Location"))
	}
	for _, language := range []string{"en", "es"} {
		data, err := os.ReadFile(filepath.Join(media, "Arrival.BluRay-GROUP."+language+".srt"))
		if err != nil || !strings.Contains(string(data), "Found") {
			t.Errorf("%s sidecar = %q, err = %v", language, data, err)
		}
	}
}

func TestSubtitleBatchAPIsDefaultToEveryPreferredLanguage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, path, result string }{{"fetch wanted", "/api/v1/subtitle-library/fetch-wanted", `"written":2`}, {"maintain", "/api/v1/subtitle-library/maintain", `"added":2`}} {
		t.Run(test.name, func(t *testing.T) {
			provider := newMultilingualSubDL(t)
			media := t.TempDir()
			writeTestFile(t, filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), "video")
			handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
			if settings := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es"]}`); settings.Code != http.StatusOK {
				t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
			}
			response := requestJSON(t, handler, http.MethodPost, test.path, `{"limit":2}`)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"attempted":2`) || !strings.Contains(response.Body.String(), test.result) {
				t.Fatalf("batch = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestProviderAliasDoesNotSatisfyLocalCoverageOrSkipMutations(t *testing.T) { //nolint:cyclop,funlen,gocognit // One fixture proves the local and provider language trust boundaries across adapters.
	for _, mutation := range []struct {
		name string
		path string
		want string
	}{{"manual", "item", ""}, {"maintenance", "/api/v1/subtitle-library/maintain", `"added":1`}} {
		t.Run(mutation.name, func(t *testing.T) {
			provider, searches := newLatinAmericanOpenSubtitles(t)
			media := t.TempDir()
			writeTestFile(t, filepath.Join(media, "Arrival.mp4"), strings.Repeat("v", 140000))
			writeTestFile(t, filepath.Join(media, "Arrival.ea.srt"), "1\n00:00:01,000 --> 00:00:02,000\nAlias\n")
			config := server.SubtitleConfig{OpenSubtitles: server.OpenSubtitlesConfig{URL: provider.URL, APIKey: "key", Username: "user", Password: "password"}}
			handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: config})
			if settings := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["es-419"]}`); settings.Code != http.StatusOK {
				t.Fatalf("settings = %d %q", settings.Code, settings.Body.String())
			}
			var inventory struct {
				Items []struct {
					ID               string   `json:"id"`
					Ready            bool     `json:"ready"`
					MissingLanguages []string `json:"missingLanguages"`
				} `json:"items"`
			}
			before := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library", "")
			if json.Unmarshal(before.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].Ready || !slices.Equal(inventory.Items[0].MissingLanguages, []string{"es-419"}) {
				t.Fatalf("inventory = %d %q", before.Code, before.Body.String())
			}
			path := mutation.path
			if path == "item" {
				path = "/api/v1/subtitle-library/" + inventory.Items[0].ID + "/fetch"
			}
			body := `{"limit":1}`
			if mutation.name == "manual" {
				body = `{}`
			}
			response := requestJSON(t, handler, http.MethodPost, path, body)
			if mutation.name == "manual" && response.Code != http.StatusCreated || mutation.name == "maintenance" && (response.Code != http.StatusOK || !strings.Contains(response.Body.String(), mutation.want) || !strings.Contains(response.Body.String(), `"upgraded":0`)) {
				t.Fatalf("mutation = %d %q", response.Code, response.Body.String())
			}
			if _, err := os.Stat(filepath.Join(media, "Arrival.es-419.srt")); err != nil || searches.Load() != 1 {
				t.Fatalf("canonical sidecar = %v, searches = %d", err, searches.Load())
			}
		})
	}
}

func newLatinAmericanOpenSubtitles(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var searches atomic.Int32
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			searches.Add(1)
			if request.URL.Query().Get("languages") != "ea" {
				http.Error(writer, "bad language", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"total_count": 1, "data": []any{map[string]any{"id": "one", "type": "subtitle", "attributes": map[string]any{"language": "ea", "moviehash_match": true, "release": "Arrival", "nb_cd": 1, "feature_details": map[string]any{"feature_type": "movie", "title": "Arrival"}, "files": []any{map[string]any{"file_id": 7, "cd_number": 1, "file_name": "Arrival.srt"}}}}}})
		case "/login":
			_ = json.NewEncoder(writer).Encode(map[string]any{"token": "session", "status": 200})
		case "/download":
			_ = json.NewEncoder(writer).Encode(map[string]any{"link": provider.URL + "/file.srt"})
		case "/file.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nCanonical\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	return provider, &searches
}

func newMultilingualSubDL(t *testing.T) *httptest.Server {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/subtitles" && request.URL.Query().Get("api_key") != "key" {
			http.Error(writer, "bad key", http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/subtitles" {
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nFound\n"))
			return
		}
		language := request.URL.Query().Get("languages")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie"}},
			"subtitles": []any{map[string]any{"url": "http://" + request.Host + "/" + strings.ToLower(language) + ".srt", "language": language, "release_name": "Arrival.BluRay-GROUP"}},
		})
	}))
	t.Cleanup(provider.Close)
	return provider
}

func writeExactOpenSubtitlesSearch(writer http.ResponseWriter) {
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
}
