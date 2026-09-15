package library_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScanBuildsRichLibraryItemsFromLocalSidecars(t *testing.T) { //nolint:cyclop,gocognit,funlen // One filesystem fixture specifies the complete public scan result.
	root := t.TempDir()
	for _, directory := range []string{"Shows/Season 1", "Music", "Audiobooks/Novel", "Photos"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	episode := filepath.Join(root, "Shows", "Season 1", "Series.S01E02.The_Return.mkv")
	for path, contents := range map[string]string{
		episode:                                                "video",
		stringsTrimExtension(episode) + ".nfo":                 `<episodedetails><title> Local title </title><sorttitle> Return, The </sorttitle><year> 2026 </year><plot> Plot </plot><mpaa> TV-14 </mpaa><contentrating> fallback </contentrating><tagline> Tagline </tagline><genre> Drama </genre><genre> Mystery </genre><director> Director </director><studio> Studio </studio></episodedetails>`,
		stringsTrimExtension(episode) + ".vtt":                 "WEBVTT",
		stringsTrimExtension(episode) + ".en.srt":              "English",
		stringsTrimExtension(episode) + ".fr.vtt":              "French",
		stringsTrimExtension(episode) + "-thumb.webp":          "thumb",
		filepath.Join(root, "Shows", "poster.png"):             "poster",
		filepath.Join(root, "Shows", "fanart.jpeg"):            "fanart",
		filepath.Join(root, "Shows", "logo.webp"):              "logo",
		filepath.Join(root, "Shows", "Season 1", "poster.jpg"): "season",
		filepath.Join(root, "Movie.mkv"):                       "movie",
		filepath.Join(root, "Movie.nfo"):                       "<invalid",
		filepath.Join(root, "Oversized.mp4"):                   "movie",
		filepath.Join(root, "Oversized.nfo"):                   `<movie><sorttitle>` + strings.Repeat("x", 513) + `</sorttitle></movie>`,
		filepath.Join(root, "Root.S02E03.mp4"):                 "episode",
		filepath.Join(root, "Music", "Song.flac"):              "song",
		filepath.Join(root, "Music", "Song.nfo"):               `<track><title> Song title </title><artist> Artist </artist><albumartist> Band </albumartist><album> Album </album><disc>2</disc><track>4</track></track>`,
		filepath.Join(root, "Music", "Song.lrc"):               "lyrics",
		filepath.Join(root, "Audiobooks", "Novel", "Part.mp3"): "audio book",
		filepath.Join(root, "Standalone.m4b"):                  "audio book",
		filepath.Join(root, "Book.pdf"):                        "book",
		filepath.Join(root, "Photos", "Vacation.gif"):          "photo",
		filepath.Join(root, "Photos", "poster.jpg"):            "sidecar artwork",
	} {
		writeLibraryFile(t, path, contents)
	}

	items, err := library.ScanContext(t.Context(), root, "Family")
	if err != nil {
		t.Fatal(err)
	}
	byTitle := make(map[string]library.Item)
	for _, item := range items {
		byTitle[item.Title] = item
	}
	local := byTitle["S01E02 · Local title"]
	if local.Show != "Shows" || local.Season != 1 || local.Episode != 2 || local.SortTitle != "Return, The" || local.Year != "2026" || local.Plot != "Plot" || local.Rating != "TV-14" || local.Genres != "Drama · Mystery" || local.Director != "Director" || local.Studio != "Studio" || local.Artwork == "" || local.ShowArtwork == "" || local.ShowBackdrop == "" || local.ShowLogo == "" || local.SeasonArtwork == "" || len(local.Subtitles) != 3 || local.Subtitle == "" || !local.LocalTitle {
		t.Fatalf("rich episode = %#v", local)
	}
	song := byTitle["Song title"]
	if song.Kind != "audio" || song.Artist != "Artist" || song.AlbumArtist != "Band" || song.Album != "Album" || song.Disc != 2 || song.Track != 4 || song.Lyrics == "" {
		t.Fatalf("music item = %#v", song)
	}
	if byTitle["Part"].Kind != "audiobook" || byTitle["Standalone"].Kind != "audiobook" || byTitle["Book"].Kind != "book" || byTitle["Vacation"].Kind != "photo" {
		t.Fatalf("media kinds = %#v", byTitle)
	}
	if _, found := byTitle["poster"]; found {
		t.Fatal("sidecar artwork was exposed as a photo")
	}
	if byTitle["Oversized"].SortTitle != "" {
		t.Fatal("oversized sort title was accepted")
	}
	if rootEpisode := byTitle["S02E03"]; rootEpisode.Show != "Root" || rootEpisode.Season != 2 || rootEpisode.Episode != 3 {
		t.Fatalf("root episode = %#v", rootEpisode)
	}
}

func TestScanHandlesEmptyAndUnavailableLibraries(t *testing.T) {
	items, err := library.ScanContext(t.Context(), "", "Family")
	if err != nil || items != nil {
		t.Fatalf("empty root = %#v, %v", items, err)
	}
	if _, err := library.ScanContext(t.Context(), filepath.Join(t.TempDir(), "missing"), "Family"); err == nil {
		t.Fatal("missing Library root was accepted")
	}
}

func TestOrganizersHandleFallbackMetadata(t *testing.T) { //nolint:cyclop // One workflow covers organizer fallback metadata.
	items := []library.Item{
		{ID: "episode-1", Kind: "video", Show: "Show", ShowTitle: "The Show", ShowArtwork: "poster", ShowBackdrop: "backdrop", ShowLogo: "logo", Season: 2, Episode: 2},
		{ID: "episode-2", Kind: "video", Show: "Show", Artwork: "fallback", SeasonArtwork: "season", Season: 2, Episode: 1},
		{ID: "album", Kind: "audio", Artist: "Artist", Album: "Album", Artwork: "cover"},
		{ID: "anonymous", Kind: "audio", Artwork: "cover"},
	}
	_, shows := library.Organize(items)
	_, albums := library.OrganizeMusic(items)
	if len(shows) != 1 || shows[0].Title != "The Show" || shows[0].Seasons[0].Artwork != "season" || shows[0].Episodes[0].ID != "episode-2" {
		t.Fatalf("shows = %#v", shows)
	}
	if len(albums) != 1 || albums[0].Artist != "Artist" || albums[0].ArtworkID != "album" {
		t.Fatalf("albums=%#v", albums)
	}
}

func stringsTrimExtension(path string) string {
	return path[:len(path)-len(filepath.Ext(path))]
}

func writeLibraryFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
