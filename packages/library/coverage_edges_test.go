package library

import "testing"

func TestOrganizationRemainingOrderingAndArtworkEdges(t *testing.T) {
	t.Parallel()
	shows := organizedShows(map[string]*Show{
		"z": {Title: "Zulu"},
		"a": {Title: "Alpha", Episodes: []Item{{Season: 1, Episode: 2, SeasonArtwork: "art"}, {Season: 1, Episode: 1, SeasonArtwork: ""}}},
	})
	if len(shows) != 2 || shows[0].Title != "Alpha" || len(shows[0].Seasons) != 1 || shows[0].Seasons[0].Artwork != "art" {
		t.Fatalf("organized shows = %#v", shows)
	}
	_, albums := OrganizeMusic([]Item{
		{Kind: "audio", Album: "Zulu", Artist: "Zulu", Disc: 1, Track: 2},
		{Kind: "audio", Album: "Alpha", Artist: "Alpha", Disc: 1, Track: 1},
	})
	if len(albums) != 2 || albums[0].Title != "Alpha" {
		t.Fatalf("organized albums = %#v", albums)
	}
}
