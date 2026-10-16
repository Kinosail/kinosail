package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type jellyfinItemPage struct {
	Items []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
		Type string `json:"Type"`
	} `json:"Items"`
	StartIndex       int `json:"StartIndex"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

func jellyfinQueryServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	mediaDir := t.TempDir()
	for _, path := range []string{
		"Arrival (2016).mp4", "Blade Runner (1982).mp4", "Holiday.png", "Song.mp3",
		"Severance/Season 01/Severance.S01E01.Good.News.mkv", "Severance/Season 01/Severance.S01E02.Half.Loop.mkv", "Severance/Season 02/Severance.S02E01.Hello.mkv",
	} {
		path = filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), RequireAuth: true})
	token, _ := jellyfinLogin(t, handler, signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password"))
	return handler, token
}

func jellyfinPage(t *testing.T, handler http.Handler, token, path string) jellyfinItemPage {
	t.Helper()
	response := jellyfinCall(t, handler, http.MethodGet, path, "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
	}
	var page jellyfinItemPage
	decodeJellyfin(t, response, &page)
	return page
}

func jellyfinPageNames(page jellyfinItemPage) []string {
	names := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		names = append(names, item.Type+":"+item.Name)
	}
	return names
}

func jellyfinPageID(t *testing.T, page jellyfinItemPage, name string) string {
	t.Helper()
	for _, item := range page.Items {
		if item.Name == name {
			return item.ID
		}
	}
	t.Fatalf("%q missing from %#v", name, page)
	return ""
}

func TestJellyfinItemsFilterAndPageTheVisibleCatalog(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinQueryServer(t)
	root := jellyfinPage(t, handler, token, "/Items")
	series := jellyfinPageID(t, root, "Severance")
	seasons := jellyfinPage(t, handler, token, "/Items?ParentId="+series)
	movie, song := jellyfinPageID(t, root, "Arrival"), jellyfinPageID(t, root, "Song")
	all := []string{"Movie:Arrival", "Movie:Blade Runner", "Series:Severance", "Photo:Holiday", "Audio:Song"}
	episodes := []string{"Episode:S01E01 · Good News", "Episode:S01E02 · Half Loop", "Episode:S02E01 · Hello"}
	for _, test := range []struct {
		path         string
		names        []string
		start, total int
	}{
		{"/Items", all, 0, 5},
		{"/Items?startIndex=-1&limit=2", all[:2], 0, 5},
		{"/Items?StartIndex=-9", all, 0, 5},
		{"/Items?StartIndex=3&Limit=10", all[3:], 3, 5},
		{"/Items?startIndex=9", []string{}, 5, 5},
		{"/Items?limit=-1", all, 0, 5},
		{"/Items?IncludeItemTypes=Episode", episodes, 0, 3},
		{"/Items?IncludeItemTypes=Episode&startIndex=1&limit=1", episodes[1:2], 1, 3},
		{"/Items?searchTerm=ARR", all[:1], 0, 1},
		{"/Items?searchTerm=e", []string{"Movie:Blade Runner", "Series:Severance"}, 0, 2},
		{"/Items?IncludeItemTypes=Movie,Series", all[:3], 0, 3},
		{"/Items?IncludeItemTypes=Audio", all[4:], 0, 1},
		{"/Items?ParentId=00000000000000000000000000000001&startIndex=1", all[1:2], 1, 2},
		{"/Items?ParentId=00000000000000000000000000000002", all[2:3], 0, 1},
		{"/Items?ParentId=00000000000000000000000000000002&IncludeItemTypes=Episode&searchTerm=half", episodes[1:2], 0, 1},
		{"/Items?ParentId=" + series, []string{"Season:Season 1", "Season:Season 2"}, 0, 2},
		{"/Items?ParentId=" + jellyfinPageID(t, seasons, "Season 2"), episodes[2:], 0, 1},
		{"/Items?Ids=" + movie + "," + song, []string{"Movie:Arrival", "Audio:Song"}, 0, 2},
		{"/Items?Ids=" + movie + "&IncludeItemTypes=Episode", []string{}, 0, 0},
		{"/Items?ParentId=unknown", []string{}, 0, 0},
	} {
		page := jellyfinPage(t, handler, token, test.path)
		if got := jellyfinPageNames(page); !reflect.DeepEqual(got, test.names) || page.StartIndex != test.start || page.TotalRecordCount != test.total {
			t.Errorf("%s = %q start %d total %d, want %q start %d total %d", test.path, got, page.StartIndex, page.TotalRecordCount, test.names, test.start, test.total)
		}
	}
	if response := jellyfinCall(t, handler, http.MethodGet, "/Items?Ids="+movie+"&Ids="+song, "", token); response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "ids") {
		t.Fatalf("duplicate ids = %d %q", response.Code, response.Body.String())
	}
}
