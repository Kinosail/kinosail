package server_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

const (
	jellyfinLimitMovies   = 1007
	jellyfinLatestDefault = 20
)

func jellyfinLimitServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	mediaDir := t.TempDir()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	write := func(name string, modified time.Time) {
		path := filepath.Join(mediaDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	for number := range jellyfinLimitMovies {
		write(fmt.Sprintf("Film %04d.mp4", number), base.Add(time.Duration(number)*time.Minute))
	}
	write("Severance/Season 01/Severance.S01E01.Good.News.mkv", base.Add(48*time.Hour))
	write("Severance/Season 01/Severance.S01E02.Half.Loop.mkv", base.Add(-time.Hour))
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), RequireAuth: true})
	token, _ := jellyfinLogin(t, handler, signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password"))
	return handler, token
}

func newestFilms(first, count int) []string {
	names := make([]string, 0, count)
	for number := first; number > first-count; number-- {
		names = append(names, fmt.Sprintf("Movie:Film %04d", number))
	}
	return names
}

func TestJellyfinItemsFollowJellyfinLimitSemantics(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinLimitServer(t)
	total := jellyfinLimitMovies + 1
	for _, test := range []struct {
		path          string
		count, start  int
		first, latest string
	}{
		{"/Items", total, 0, "Movie:Film 0000", "Series:Severance"},
		{"/Items?limit=", total, 0, "Movie:Film 0000", "Series:Severance"},
		{"/Items?limit=-1", total, 0, "Movie:Film 0000", "Series:Severance"},
		{"/Items?Limit=2147483647", total, 0, "Movie:Film 0000", "Series:Severance"},
		{"/Items?limit=1001", 1001, 0, "Movie:Film 0000", "Movie:Film 1000"},
		{"/Items?limit=3&startIndex=1004", 3, 1004, "Movie:Film 1004", "Movie:Film 1006"},
		{"/Items?startIndex=1000&limit=5000", total - 1000, 1000, "Movie:Film 1000", "Series:Severance"},
		{"/Items?limit=0", 0, 0, "", ""},
	} {
		page := jellyfinPage(t, handler, token, test.path)
		names := jellyfinPageNames(page)
		if len(names) != test.count || page.StartIndex != test.start || page.TotalRecordCount != total {
			t.Errorf("%s = %d items start %d total %d, want %d start %d total %d", test.path, len(names), page.StartIndex, page.TotalRecordCount, test.count, test.start, total)
			continue
		}
		if test.count > 0 && (names[0] != test.first || names[len(names)-1] != test.latest) {
			t.Errorf("%s = %q..%q, want %q..%q", test.path, names[0], names[len(names)-1], test.first, test.latest)
		}
	}
}

func TestJellyfinLatestReturnsANewestFirstSelection(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinLimitServer(t)
	newest := jellyfinLimitMovies - 1
	for _, test := range []struct {
		path  string
		names []string
	}{
		{"/Items/Latest", newestFilms(newest, jellyfinLatestDefault)},
		{"/Items/Latest?limit=-1", newestFilms(newest, 1)},
		{"/Items/Latest?limit=3", newestFilms(newest, 3)},
		{"/Items/Latest?limit=2147483647", append(newestFilms(newest, jellyfinLimitMovies), "Series:Severance")},
		{"/Items/Latest?Limit=0", []string{}},
		{"/Items/Latest?ParentId=00000000000000000000000000000001&IncludeItemTypes=Movie&limit=2", newestFilms(newest, 2)},
		{"/Items/Latest?searchTerm=film%20000&limit=2", newestFilms(9, 2)},
		{"/Items/Latest?IncludeItemTypes=Episode&limit=5", []string{"Episode:S01E01 · Good News", "Episode:S01E02 · Half Loop"}},
		{"/Items/Latest?ParentId=00000000000000000000000000000002", []string{"Series:Severance"}},
	} {
		response := jellyfinCall(t, handler, http.MethodGet, test.path, "", token)
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d %q", test.path, response.Code, response.Body.String())
		}
		var page jellyfinItemPage
		decodeJellyfin(t, response, &page.Items)
		if got := jellyfinPageNames(page); !reflect.DeepEqual(got, test.names) {
			t.Errorf("%s = %d items %q, want %d items %q", test.path, len(got), got, len(test.names), test.names)
		}
	}
}

func TestJellyfinListsRejectLimitsJellyfinCannotParse(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinLimitServer(t)
	for _, route := range []string{"/Items", "/Items/Latest"} {
		for _, limit := range []string{"abc", "1.5", "2147483648", "99999999999999999999", "-2147483649"} {
			path := route + "?limit=" + limit
			response := jellyfinCall(t, handler, http.MethodGet, path, "", token)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "limit") {
				t.Errorf("%s = %d %q, want 400 naming the limit", path, response.Code, response.Body.String())
			}
		}
	}
}
