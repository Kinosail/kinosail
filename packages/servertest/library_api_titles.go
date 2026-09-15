package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (fixture LibraryAPIFixture) KnownTitleSearchRanksLikelyMatchesAcrossAPIAndWeb(t *testing.T) {
	mediaDir := t.TempDir()
	for _, name := range []string{"A Story About Grinch.mp4", "Grinch Behind the Scenes.mp4", "Grinch.mp4", "The Grinch (2000).mp4", "WALL-E.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&q=Grinch", nil)
	assertKnownTitleRanking(t, response)
	normalized := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&q=wall+e", nil)
	AssertAPIBody(t, normalized, http.StatusOK, `"title":"WALL E"`)
	punctuation := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&q=%21%21%21", nil)
	AssertAPIBody(t, punctuation, http.StatusOK, `"total":0`)
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&q=Grinch", nil))
	body := web.Body.String()
	if web.Code != http.StatusOK || !strings.Contains(body, "4 results for") || !strings.Contains(body, `data-search-clear`) || strings.Index(body, ">Grinch</h2>") > strings.Index(body, ">Grinch Behind the Scenes</h2>") {
		t.Fatalf("web search = %d %q", web.Code, body)
	}
}

func assertKnownTitleRanking(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var catalog struct {
		Items []struct{ Title string } `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK {
		t.Fatalf("search = %d %q: %v", response.Code, response.Body.String(), err)
	}
	want := []string{"Grinch", "Grinch Behind the Scenes", "The Grinch", "A Story About Grinch"}
	if len(catalog.Items) != len(want) {
		t.Fatalf("search items = %#v", catalog.Items)
	}
	for position, title := range want {
		if catalog.Items[position].Title != title {
			t.Fatalf("search item %d = %q, want %q", position, catalog.Items[position].Title, title)
		}
	}
}

func (fixture LibraryAPIFixture) ExplicitSortTitleControlsBrowseWithoutChangingDisplayTitle(t *testing.T) {
	media := t.TempDir()
	for name, content := range map[string]string{
		"Home.mp4":       "video",
		"The Grinch.mp4": "video",
		"The Grinch.nfo": `<movie><title>The Grinch</title><sorttitle>Grinch, The</sorttitle></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(media, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&letter=G", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"The Grinch"`) || !strings.Contains(response.Body.String(), `"sortTitle":"Grinch, The"`) || strings.Contains(response.Body.String(), `"title":"Home"`) {
		t.Fatalf("sort title browse = %d %q", response.Code, response.Body.String())
	}
}

func (fixture LibraryAPIFixture) ReleaseFilenameUsesHumanTitleInAPIAndWeb(t *testing.T) {
	mediaDir := t.TempDir()
	name := "'Twas the Night Before Christmas (1974) {imdb tt0208654} [Bluray 1080p] [DTS 1 0][x264] SADPANDA.mkv"
	if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID, Title, Year string
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK || len(catalog.Items) != 1 {
		t.Fatalf("API library = %d %q: %v", response.Code, response.Body.String(), err)
	}
	item := catalog.Items[0]
	if item.Title != "'Twas the Night Before Christmas" || item.Year != "1974" {
		t.Fatalf("API title = %q, year = %q", item.Title, item.Year)
	}
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+item.ID, nil))
	if player.Code != http.StatusOK || !strings.Contains(player.Body.String(), "<h1>&#39;Twas the Night Before Christmas <small>1974</small></h1>") || strings.Contains(player.Body.String(), "Bluray 1080p") {
		t.Fatalf("player = %d %q", player.Code, player.Body.String())
	}
}
