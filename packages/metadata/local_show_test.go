package metadata

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestApplyKeepsShowTextFromTVShowNFO(t *testing.T) {
	t.Parallel()
	record := Record{Title: "S01E01 · Provider", ShowTitle: "Provider Show", ShowYear: "2001", ShowPlot: "Provider plot", ShowArtwork: "poster", ShowProviderIDs: map[string]string{"tmdb": "7"}}
	local := library.Item{Title: "S01E01", ShowTitle: "Local Show", ShowYear: "2019", LocalShowMetadata: true}
	apply(&local, record)
	if local.ShowTitle != "Local Show" || local.ShowYear != "2019" || local.ShowPlot != "Provider plot" || local.ShowArtwork != "poster" || local.Title != "S01E01 · Provider" || local.ShowProviderIDs["tmdb"] != "7" {
		t.Fatalf("item with tvshow.nfo = %+v", local)
	}
	plain := library.Item{Title: "S01E01", ShowTitle: "Folder Show"}
	apply(&plain, record)
	if plain.ShowTitle != "Provider Show" || plain.ShowYear != "2001" || plain.ShowPlot != "Provider plot" {
		t.Fatalf("item without tvshow.nfo = %+v", plain)
	}
}
