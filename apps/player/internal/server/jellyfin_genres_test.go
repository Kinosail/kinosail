package server_test

import (
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestJellyfinGenresAndFiltersDescribeTheVisibleCatalog(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinSortServer(t)
	movies := "ParentId=00000000000000000000000000000001"
	genres := []string{"Genre:Crime", "Genre:Drama", "Genre:Mystery", "Genre:Science Fiction"}
	for _, test := range []struct {
		path  string
		names []string
		total int
	}{
		{"/Genres?" + movies, genres, 4},
		{"/Genres", genres, 4},
		{"/Genres?" + movies + "&StartIndex=1&Limit=2", genres[1:3], 4},
		{"/Genres?" + movies + "&SearchTerm=sci", genres[3:], 1},
		{"/Genres?ParentId=00000000000000000000000000000002", []string{}, 0},
		{"/Genres?ParentId=missing", []string{}, 0},
	} {
		page := jellyfinPage(t, handler, token, test.path)
		if got := jellyfinPageNames(page); !reflect.DeepEqual(got, test.names) || page.TotalRecordCount != test.total {
			t.Errorf("%s = %q total %d, want %q total %d", test.path, got, page.TotalRecordCount, test.names, test.total)
		}
		for _, genre := range page.Items {
			if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(genre.ID) {
				t.Errorf("%s genre id = %q", test.path, genre.ID)
			}
		}
	}
	listed := jellyfinPage(t, handler, token, "/Genres?"+movies)
	mystery, crime := jellyfinPageID(t, listed, "Mystery"), jellyfinPageID(t, listed, "Crime")
	for path, names := range map[string][]string{
		"/Items?" + movies + "&GenreIds=" + mystery:                                 {"Movie:Arrival", "Movie:Zodiac"},
		"/Items?" + movies + "&genreIds=" + crime + "%7C" + strings.Repeat("0", 32): {"Movie:Zodiac"},
		"/Items?" + movies + "&GenreIds=" + strings.Repeat("0", 32):                 {},
		"/Items?" + movies + "&GenreIds=" + mystery + "&Genres=Drama":               {"Movie:Arrival"},
	} {
		if got := jellyfinPageNames(jellyfinPage(t, handler, token, path)); !reflect.DeepEqual(got, names) {
			t.Errorf("%s = %q, want %q", path, got, names)
		}
	}
	var filters struct {
		Genres          []string
		Tags            []string
		OfficialRatings []string
		Years           []int
	}
	decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Items/Filters?"+movies, "", token), &filters)
	if !reflect.DeepEqual(filters.Genres, []string{"Crime", "Drama", "Mystery", "Science Fiction"}) || !reflect.DeepEqual(filters.Years, []int{1982, 2007, 2016}) || filters.Tags == nil || filters.OfficialRatings == nil {
		t.Errorf("filters = %+v", filters)
	}
	for _, path := range []string{
		"/Items?" + movies + "&GenreIds=mystery", "/Items?" + movies + "&GenreIds=" + strings.Repeat(mystery+"%7C", 16) + mystery,
		"/Genres?Limit=many", "/Genres?SearchTerm=" + strings.Repeat("x", 65),
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", path[:min(len(path), 70)], response.Code)
		}
	}
	for _, path := range []string{"/Genres", "/Items/Filters"} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", ""); response.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s = %d, want 401", path, response.Code)
		}
	}
}
