package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func jellyfinSortServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	mediaDir := t.TempDir()
	added := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for position, file := range []struct{ path, content string }{
		{"Zodiac (2007).mp4", "media"},
		{"Zodiac (2007).nfo", `<movie><genre>Crime</genre><genre>Mystery</genre></movie>`},
		{"Arrival (2016).mp4", "media"},
		{"Arrival (2016).nfo", `<movie><genre>Drama</genre><genre>Mystery</genre></movie>`},
		{"Blade Runner (1982).mp4", "media"},
		{"Blade Runner (1982).nfo", `<movie><genre>Science Fiction</genre></movie>`},
		{"Untagged.mp4", "media"},
		{"Severance/Season 01/Severance.S01E01.Good.News.mkv", "media"},
	} {
		path := filepath.Join(mediaDir, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.content), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := added.Add(time.Duration(position) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), RequireAuth: true})
	token, _ := jellyfinLogin(t, handler, signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password"))
	return handler, token
}

func TestJellyfinItemsSortAndFilterByGenreYearAndName(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinSortServer(t)
	movies := "/Items?ParentId=00000000000000000000000000000001"
	byName := []string{"Movie:Arrival", "Movie:Blade Runner", "Movie:Untagged", "Movie:Zodiac"}
	reversed := []string{"Movie:Zodiac", "Movie:Untagged", "Movie:Blade Runner", "Movie:Arrival"}
	for _, test := range []struct {
		path         string
		names        []string
		start, total int
	}{
		{movies, byName, 0, 4},
		{movies + "&SortBy=SortName", byName, 0, 4},
		{movies + "&SortBy=SortName&SortOrder=Descending", reversed, 0, 4},
		{movies + "&sortBy=Name&sortOrder=descending&startIndex=1&limit=2", reversed[1:3], 1, 4},
		{movies + "&SortBy=DateCreated&SortOrder=Descending", []string{"Movie:Untagged", "Movie:Blade Runner", "Movie:Arrival", "Movie:Zodiac"}, 0, 4},
		{movies + "&SortBy=DateCreated,SortName", []string{"Movie:Zodiac", "Movie:Arrival", "Movie:Blade Runner", "Movie:Untagged"}, 0, 4},
		{movies + "&SortBy=ProductionYear,SortName", []string{"Movie:Untagged", "Movie:Blade Runner", "Movie:Zodiac", "Movie:Arrival"}, 0, 4},
		{movies + "&SortBy=PremiereDate&SortOrder=Descending", []string{"Movie:Arrival", "Movie:Zodiac", "Movie:Blade Runner", "Movie:Untagged"}, 0, 4},
		{movies + "&SortBy=Random", byName, 0, 4},
		{movies + "&SortBy=CommunityRating,SortName&SortOrder=Descending", reversed, 0, 4},
		{movies + "&Genres=Mystery", []string{"Movie:Arrival", "Movie:Zodiac"}, 0, 2},
		{movies + "&genres=crime%7Cdrama", []string{"Movie:Arrival", "Movie:Zodiac"}, 0, 2},
		{movies + "&Genres=Science%20Fiction", []string{"Movie:Blade Runner"}, 0, 1},
		{movies + "&Genres=Western", []string{}, 0, 0},
		{movies + "&Genres=Mystery&SortBy=SortName&SortOrder=Descending&Limit=1", []string{"Movie:Zodiac"}, 0, 2},
		{movies + "&Years=1982,2016", []string{"Movie:Arrival", "Movie:Blade Runner"}, 0, 2},
		{movies + "&Years=2007&Genres=Mystery", []string{"Movie:Zodiac"}, 0, 1},
		{movies + "&Years=1999", []string{}, 0, 0},
		{movies + "&NameStartsWith=b", []string{"Movie:Blade Runner"}, 0, 1},
		{movies + "&NameStartsWith=Z&Genres=Crime", []string{"Movie:Zodiac"}, 0, 1},
		{"/Items?Genres=Mystery", []string{"Movie:Arrival", "Movie:Zodiac"}, 0, 2},
		{"/Items?IncludeItemTypes=Movie,Series&SortBy=SortName&SortOrder=Descending", []string{"Movie:Zodiac", "Movie:Untagged", "Series:Severance", "Movie:Blade Runner", "Movie:Arrival"}, 0, 5},
		{"/Items?IncludeItemTypes=Series&Years=2016", []string{}, 0, 0},
	} {
		page := jellyfinPage(t, handler, token, test.path)
		if got := jellyfinPageNames(page); !reflect.DeepEqual(got, test.names) || page.StartIndex != test.start || page.TotalRecordCount != test.total {
			t.Errorf("%s = %q start %d total %d, want %q start %d total %d", test.path, got, page.StartIndex, page.TotalRecordCount, test.names, test.start, test.total)
		}
	}
	for _, query := range []string{
		"SortOrder=Sideways", "Years=82", "Years=2016,soon", "Years=" + strings.Repeat("2016,", 33) + "2016",
		"Genres=" + strings.Repeat("x", 65), "Genres=" + strings.Repeat("a%7C", 17) + "a", "NameStartsWith=" + strings.Repeat("x", 65), "SortBy=" + strings.Repeat("x", 257),
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, movies+"&"+query, "", token); response.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %q, want 400", query[:min(len(query), 40)], response.Code, response.Body.String())
		}
	}
}
