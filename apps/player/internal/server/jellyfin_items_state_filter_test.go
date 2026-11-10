package server_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestJellyfinItemsFilterByPlayedAndFavoriteState(t *testing.T) {
	t.Parallel()
	handler, token := jellyfinSortServer(t)
	movies := "/Items?ParentId=00000000000000000000000000000001"
	all := jellyfinPage(t, handler, token, movies)
	for _, path := range []string{
		"/UserPlayedItems/" + jellyfinPageID(t, all, "Arrival"),
		"/UserFavoriteItems/" + jellyfinPageID(t, all, "Zodiac"),
		"/UserFavoriteItems/" + jellyfinPageID(t, all, "Arrival"),
	} {
		if response := jellyfinCall(t, handler, http.MethodPost, path, "", token); response.Code != http.StatusOK {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	unplayed := []string{"Movie:Blade Runner", "Movie:Untagged", "Movie:Zodiac"}
	for _, test := range []struct {
		path  string
		names []string
	}{
		{movies + "&Filters=IsPlayed", []string{"Movie:Arrival"}},
		{movies + "&Filters=IsUnplayed", unplayed},
		{movies + "&filters=isunplayed&SortBy=SortName&SortOrder=Descending&Limit=2", []string{"Movie:Zodiac", "Movie:Untagged"}},
		{movies + "&Filters=IsFavorite", []string{"Movie:Arrival", "Movie:Zodiac"}},
		{movies + "&Filters=IsFavorite,IsUnplayed", []string{"Movie:Zodiac"}},
		{movies + "&Filters=IsPlayed,IsUnplayed", []string{}},
		{movies + "&Filters=IsFolder,Likes", []string{"Movie:Arrival", "Movie:Blade Runner", "Movie:Untagged", "Movie:Zodiac"}},
		{movies + "&IsPlayed=true", []string{"Movie:Arrival"}},
		{movies + "&isPlayed=false", unplayed},
		{movies + "&IsFavorite=true&IsPlayed=false", []string{"Movie:Zodiac"}},
		{movies + "&IsFavorite=false", []string{"Movie:Blade Runner", "Movie:Untagged"}},
		{movies + "&Filters=IsFavorite&Genres=Drama", []string{"Movie:Arrival"}},
		{"/Items?IncludeItemTypes=Series&Filters=IsPlayed", []string{}},
		{"/Items?IncludeItemTypes=Series&Filters=IsUnplayed", []string{"Series:Severance"}},
		{"/Items?IncludeItemTypes=Movie,Series&Filters=IsFavorite", []string{"Movie:Arrival", "Movie:Zodiac"}},
	} {
		page := jellyfinPage(t, handler, token, test.path)
		if got := jellyfinPageNames(page); !reflect.DeepEqual(got, test.names) || (!strings.Contains(test.path, "Limit=") && page.TotalRecordCount != len(test.names)) {
			t.Errorf("%s = %q total %d, want %q", test.path, got, page.TotalRecordCount, test.names)
		}
	}
	episodes := jellyfinPage(t, handler, token, "/Items?IncludeItemTypes=Episode")
	if len(episodes.Items) != 1 {
		t.Fatalf("episodes = %+v", episodes)
	}
	if response := jellyfinCall(t, handler, http.MethodPost, "/UserPlayedItems/"+episodes.Items[0].ID, "", token); response.Code != http.StatusOK {
		t.Fatalf("mark episode played = %d %q", response.Code, response.Body.String())
	}
	if got := jellyfinPageNames(jellyfinPage(t, handler, token, "/Items?IncludeItemTypes=Series&Filters=IsPlayed")); !reflect.DeepEqual(got, []string{"Series:Severance"}) {
		t.Errorf("played series = %q", got)
	}
	for _, query := range []string{"IsPlayed=maybe", "IsFavorite=1", "Filters=" + strings.Repeat("x", 257)} {
		if response := jellyfinCall(t, handler, http.MethodGet, movies+"&"+query, "", token); response.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %q, want 400", query[:min(len(query), 40)], response.Code, response.Body.String())
		}
	}
}
