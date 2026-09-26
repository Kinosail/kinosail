package catalogapi

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestProjectItemUsesFriendlyShowNameWithoutChangingIdentity(t *testing.T) {
	item := library.Item{
		ID: "episode", Kind: "video", Title: "S01E01 · Pilot",
		Show: "Age of Attraction (2026) {imdb tt1234567}", ShowTitle: "Age of Attraction",
		Season: 1, Episode: 1, Artwork: "episode.jpg",
	}
	progress := catalog.PlaybackState{Seconds: 44}
	result := ProjectItem(item, progress, ItemAccess{Stream: true, Download: true})
	if result.Show != "Age of Attraction" || result.Title != item.Title {
		t.Fatalf("friendly labels = %#v", result)
	}
	if result.ShowID != library.ShowID(item.Show) || result.ID != item.ID || result.Progress != progress ||
		result.Stream != "/media/episode" || result.Download != "/download/episode" || result.Artwork != "/art/episode?variant=episode" {
		t.Fatalf("identity or playback changed: %#v", result)
	}
	if item.Show != "Age of Attraction (2026) {imdb tt1234567}" {
		t.Fatal("projection changed the source identity")
	}
}

func TestProjectItemWithoutShowMetadataKeepsExistingLabels(t *testing.T) {
	for _, item := range []library.Item{
		{ID: "episode", Kind: "video", Title: "S01E01 · Pilot", Show: "Series"},
		{ID: "movie", Kind: "video", Title: "1917"},
		{ID: "track", Kind: "audio", Title: "Track", Artist: "Artist"},
	} {
		result := ProjectItem(item, catalog.PlaybackState{}, ItemAccess{})
		if result.Title != item.Title || result.Show != item.Show || result.Artist != item.Artist {
			t.Fatalf("labels changed without show metadata: %#v", result)
		}
	}
}

func TestEpisodeWithoutSuppliedArtworkExposesItsOwnStill(t *testing.T) {
	for _, item := range []library.Item{
		{ID: "special", Kind: "video", Show: "Series", Season: 0, Episode: 0},
		{ID: "first", Kind: "video", Show: "Series", Season: 1, Episode: 1, ShowBackdrop: "show.jpg"},
		{ID: "second", Kind: "video", Show: "Series", Season: 1, Episode: 2, ShowBackdrop: "show.jpg"},
	} {
		result := ProjectItem(item, catalog.PlaybackState{}, ItemAccess{})
		if result.Artwork != "/episode-art/"+item.ID {
			t.Fatalf("episode %s artwork = %q", item.ID, result.Artwork)
		}
	}
	for _, item := range []library.Item{
		{ID: "movie", Kind: "video"},
		{ID: "track", Kind: "audio", Show: "Series", Episode: 1},
	} {
		if result := ProjectItem(item, catalog.PlaybackState{}, ItemAccess{}); result.Artwork != "" {
			t.Fatalf("non-episode %s artwork = %q", item.ID, result.Artwork)
		}
	}
}
