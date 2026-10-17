package server_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

type sidecarRefreshItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Ready bool   `json:"ready"`
}

func TestSidecarWritesListNewTracksWithoutRescanningOtherFolders(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/subtitles":
			writeSubDLSearch(writer, request.Host, "/subtitle.srt", request.URL.Query().Get("film_name"))
		case "/subtitle.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nFound\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	for _, title := range []string{"Alpha", "Bravo", "Charlie"} {
		writeTestFile(t, filepath.Join(media, "Movies", title, title+".BluRay-GROUP.mp4"), "video")
	}
	writeTestFile(t, filepath.Join(media, "Shows", "Delta.BluRay-GROUP.mp4"), "video")
	writeTestFile(t, filepath.Join(media, "Extras", "notes.txt"), "notes")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
	ids := make(map[string]string)
	for _, item := range sidecarRefreshInventory(t, handler) {
		ids[strings.Fields(item.Title)[0]] = item.ID
	}
	writeTestFile(t, filepath.Join(media, "Extras", "Echo.BluRay-GROUP.mp4"), "video")
	fetched := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+ids["Alpha"]+"/fetch", `{"language":"en"}`)
	edit := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte("1\n00:00:01,000 --> 00:00:02,000\nEdited\n")) + `"}`
	applied := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+ids["Bravo"]+"/apply", edit)
	maintained := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/maintain", `{"limit":50}`)
	if fetched.Code != http.StatusCreated || applied.Code != http.StatusOK || maintained.Code != http.StatusOK {
		t.Fatalf("fetch = %d %q, apply = %d %q, maintain = %d %q", fetched.Code, fetched.Body.String(), applied.Code, applied.Body.String(), maintained.Code, maintained.Body.String())
	}
	if !strings.Contains(applied.Body.String(), "Edited") {
		t.Fatalf("apply response does not show the saved subtitle: %s", applied.Body.String())
	}
	items := sidecarRefreshInventory(t, handler)
	for _, item := range items {
		if !item.Ready {
			t.Fatalf("%s does not list its new subtitle after the write", item.Title)
		}
	}
	if len(items) != 4 {
		t.Fatalf("sidecar writes rescanned unrelated folders: %d items, want 4", len(items))
	}
}

func sidecarRefreshInventory(t *testing.T, handler http.Handler) []sidecarRefreshItem {
	t.Helper()
	var inventory struct {
		Items []sidecarRefreshItem `json:"items"`
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil {
		t.Fatalf("inventory = %d %q", response.Code, response.Body.String())
	}
	return inventory.Items
}
