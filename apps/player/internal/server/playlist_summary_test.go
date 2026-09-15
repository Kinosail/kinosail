package server

import (
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestPlaylistSummariesDescribeRulesAndArtwork(t *testing.T) {
	t.Parallel()

	store := newListStore("")
	store.playlists["owner:Manual"] = map[string]bool{"art": true, "plain": true}
	store.playlistOrder["owner:Manual"] = []string{"art", "plain"}
	store.smart["owner:Everything"] = playlistRule{}
	store.smart["owner:Drama"] = playlistRule{Query: "arrival", Kind: "video"}
	items := []library.Item{
		{ID: "art", Title: "Arrival", Kind: "video", Artwork: "poster"},
		{ID: "plain", Title: "Notes", Kind: "book"},
	}

	summaries := store.PlaylistSummaries(ownerRequest("/"), items, "")
	if len(summaries) != 3 {
		t.Fatalf("summaries = %#v", summaries)
	}
	assertPlaylistSummaryRules(t, summaries)
	if filtered := store.PlaylistSummaries(ownerRequest("/"), items, "missing"); len(filtered) != 0 {
		t.Fatalf("query unexpectedly matched: %#v", filtered)
	}
}

func assertPlaylistSummaryRules(t *testing.T, summaries []playlistSummary) {
	t.Helper()
	byName := make(map[string]playlistSummary, len(summaries))
	for _, summary := range summaries {
		byName[summary.Name] = summary
	}
	if manual := byName["Manual"]; manual.Mode != "Manual" || !reflect.DeepEqual(manual.ArtworkIDs, []string{"art"}) || len(manual.Preview) != 2 {
		t.Fatalf("manual summary = %#v", manual)
	}
	if everything := byName["Everything"]; everything.Mode != "Smart" || everything.Rule != "All media" {
		t.Fatalf("all-media summary = %#v", everything)
	}
	if drama := byName["Drama"]; drama.Mode != "Smart" || drama.Rule != `matching "arrival" · video` || drama.ItemCount != 1 {
		t.Fatalf("filtered summary = %#v", drama)
	}
}
