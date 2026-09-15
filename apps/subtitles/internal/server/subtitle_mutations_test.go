package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleMutationsRejectInvalidInputBeforeProviderOrFilesystem(t *testing.T) { //nolint:cyclop,funlen // One table proves every malformed transport case stays side-effect free.
	t.Parallel()
	var searches atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		searches.Add(1)
		http.Error(writer, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	home := requestApp(t, handler, http.MethodGet, "/", "")
	id := regexp.MustCompile(`/subtitles/manage/([a-f0-9]+)/fetch`).FindStringSubmatch(home.Body.String())[1]

	requests := []struct {
		path, body string
	}{
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":"english"}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":""}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":null}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":"en","unknown":true}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":"en","language":"es"}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", ``},
		{"/api/v1/subtitle-library/" + id + "/fetch", `[]`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":{}}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{} {}`},
		{"/api/v1/subtitle-library/" + id + "/fetch", `{"language":"` + strings.Repeat("x", 4097) + `"}`},
		{"/api/v1/subtitle-library/bad!/fetch", `{}`},
		{"/api/v1/subtitle-library/" + strings.Repeat("a", 17) + "/fetch", `{}`},
		{"/api/v1/subtitle-library/fetch-wanted", `{"limit":51}`},
		{"/api/v1/subtitle-library/fetch-wanted", `{"limit":0}`},
		{"/api/v1/subtitle-library/fetch-wanted", `{"limit":null}`},
		{"/api/v1/subtitle-library/fetch-wanted", `{"limit":1,"unknown":true}`},
		{"/api/v1/subtitle-library/fetch-wanted", "{"},
		{"/api/v1/subtitle-library/maintain", `{"limit":51}`},
		{"/api/v1/subtitle-library/maintain", `{"limit":0}`},
		{"/api/v1/subtitle-library/maintain", `{"limit":null}`},
		{"/api/v1/subtitle-library/maintain", `{"language":"english"}`},
		{"/api/v1/subtitle-library/maintain", `{"limit":1,"unknown":true}`},
	}
	for _, test := range requests {
		response := requestJSON(t, handler, http.MethodPost, test.path, test.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s = %d %q", test.path, test.body, response.Code, response.Body.String())
		}
	}
	unknown := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/ffffffffffffffff/fetch", `{}`)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d %q", unknown.Code, unknown.Body.String())
	}
	if searches.Load() != 0 {
		t.Fatalf("invalid input caused %d provider requests", searches.Load())
	}
	if _, err := os.Stat(filepath.Join(media, "Arrival.en.srt")); !os.IsNotExist(err) {
		t.Fatalf("invalid input created a sidecar: %v", err)
	}
	webInvalid := requestApp(t, handler, http.MethodPost, "/subtitles/manage/"+id+"/fetch", "x")
	webFailure := requestApp(t, handler, http.MethodPost, "/subtitles/manage/"+id+"/fetch", "")
	webBatchFailure := requestApp(t, handler, http.MethodPost, "/subtitles/manage/fetch-wanted", "")
	failure := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{}`)
	batchFailure := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/fetch-wanted", `{"limit":1}`)
	if webInvalid.Code != http.StatusBadRequest || webFailure.Code != http.StatusBadGateway || webBatchFailure.Code != http.StatusBadGateway || failure.Code != http.StatusBadGateway || batchFailure.Code != http.StatusBadGateway || searches.Load() != 1 {
		t.Fatalf("provider failures = invalid %d, web %d, web batch %d, api %d, api batch %d, searches %d", webInvalid.Code, webFailure.Code, webBatchFailure.Code, failure.Code, batchFailure.Code, searches.Load())
	}
}

func TestSubtitleWebAndBatchMutationsWriteWantedSidecars(t *testing.T) { //nolint:cyclop // Web, API, and batch adapters must prove the same write operation.
	t.Parallel()
	var searches atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			searches.Add(1)
			writeSubDLSearch(writer, request.Host, "/subtitle.srt", request.URL.Query().Get("film_name"))
		case "/subtitle.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nFound\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	for _, title := range []string{"Alpha", "Bravo", "Charlie", "Delta"} {
		writeTestFile(t, filepath.Join(media, title+".BluRay-GROUP.mp4"), "video")
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	var inventory struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 4 {
		t.Fatalf("inventory = %d %q", response.Code, response.Body.String())
	}
	web := requestApp(t, handler, http.MethodPost, "/subtitles/manage/"+inventory.Items[0].ID+"/fetch", "")
	alias := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+inventory.Items[1].ID+"/fetch", `{"language":"eng"}`)
	batch := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/fetch-wanted", `{"limit":1}`)
	webBatch := requestApp(t, handler, http.MethodPost, "/subtitles/manage/fetch-wanted", "")
	if web.Code != http.StatusSeeOther || alias.Code != http.StatusCreated || batch.Code != http.StatusOK || webBatch.Code != http.StatusSeeOther || searches.Load() != 4 {
		t.Fatalf("web = %d, alias = %d %q, batch = %d %q, web batch = %d, searches = %d", web.Code, alias.Code, alias.Body.String(), batch.Code, batch.Body.String(), webBatch.Code, searches.Load())
	}
	for _, title := range []string{"Alpha", "Bravo", "Charlie", "Delta"} {
		if data, err := os.ReadFile(filepath.Join(media, title+".BluRay-GROUP.en.srt")); err != nil || !strings.Contains(string(data), "Found") {
			t.Fatalf("%s sidecar = %q, %v", title, data, err)
		}
	}
}
