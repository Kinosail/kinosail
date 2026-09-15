package library

import (
	"testing"
)

func TestOrganizersBuildVideoAndMusicHierarchies(t *testing.T) { //nolint:cyclop // One compact fixture verifies both hierarchy builders.
	items := []Item{
		{ID: "movie", Kind: "video", Title: "Movie"},
		{ID: "e2", Kind: "video", Title: "Episode 2", Show: "Show", Season: 1, Episode: 2},
		{ID: "e1", Kind: "video", Title: "Episode 1", Show: "Show", ShowTitle: "The Show", ShowYear: "2024", Season: 1, Episode: 1, Artwork: "still", ShowArtwork: "poster", ShowBackdrop: "backdrop", ShowLogo: "logo", SeasonArtwork: "season"},
		{ID: "track2", Kind: "audio", Title: "Track 2", Artist: "Artist", AlbumArtist: "Band", Album: "Album", Disc: 1, Track: 2},
		{ID: "track1", Kind: "audio", Title: "Track 1", Artist: "Artist", AlbumArtist: "Band", Album: "Album", Disc: 1, Track: 1, Artwork: "cover"},
		{ID: "loose", Kind: "audio", Title: "Single", Artist: "Solo"},
	}
	movies, shows := Organize(items)
	loose, albums := OrganizeMusic(items)
	if len(movies) != 1 || len(shows) != 1 || shows[0].Title != "The Show" || shows[0].Year != "2024" || shows[0].Episodes[0].ID != "e1" || shows[0].ArtworkID != "e1" || shows[0].Backdrop != "backdrop" || shows[0].Logo != "logo" || len(shows[0].Seasons) != 1 || shows[0].Seasons[0].Artwork != "season" {
		t.Fatalf("movies=%v shows=%v", movies, shows)
	}
	if len(loose) != 1 || len(albums) != 1 || albums[0].Tracks[0].ID != "track1" || albums[0].Artist != "Band" {
		t.Fatalf("loose=%v albums=%v", loose, albums)
	}
}
