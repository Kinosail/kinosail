package jellyfincompat

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCatalogProjectsViewsItemsAndHierarchy(t *testing.T) { //nolint:cyclop,funlen,gocognit // One catalog exercises every supported Jellyfin browse shape.
	t.Parallel()
	items := catalogFixture()
	catalog := fixtureCatalog(items)

	views := catalog.Views()
	assertResultCount(t, views, 2)
	assertResultCount(t, NewCatalog(nil, fixtureProject).Views(), 0)

	if _, err := catalog.Items(url.Values{"IDs": {"one", "two"}}); err == nil {
		t.Fatal("duplicate ids query succeeded")
	}
	result, err := catalog.Items(url.Values{"ParentId": {MoviesID}, "IncludeItemTypes": {"Movie"}, "SearchTerm": {"MOVIE"}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 1)

	result, err = catalog.Items(url.Values{"ParentId": {ShowsID}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 1)

	result, err = catalog.Items(url.Values{"ParentId": {ShowsID}, "IncludeItemTypes": {"Episode"}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 2)

	result, err = catalog.Items(url.Values{"IncludeItemTypes": {"Episode"}, "Limit": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 1)

	result, err = catalog.Items(url.Values{"StartIndex": {"999"}, "Limit": {"invalid"}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 0)
	if result["StartIndex"] != 3 || result["TotalRecordCount"] != 3 {
		t.Fatalf("paged result = %#v", result)
	}

	showID := ShowID("Raw Show")
	result, err = catalog.Items(url.Values{"ParentId": {ID(showID)}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 2)

	result, err = catalog.Items(url.Values{"ParentId": {SeasonID(showID, 2)}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 1)

	result, err = catalog.Items(url.Values{"ParentId": {"missing"}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 0)

	result, err = catalog.Items(url.Values{"Ids": {ID(items[0].ID)}})
	if err != nil {
		t.Fatal(err)
	}
	assertResultCount(t, result, 1)

	for _, id := range []string{items[0].ID, ID(showID), showID, SeasonID(showID, 1)} {
		if _, found := catalog.Item(id); !found {
			t.Fatalf("item %q not found", id)
		}
	}
	if _, found := catalog.Item("missing"); found {
		t.Fatal("missing item found")
	}

	seasons, found := catalog.SeasonsResult(ID(showID))
	if !found {
		t.Fatal("series seasons not found")
	}
	assertResultCount(t, seasons, 2)
	if _, found = catalog.SeasonsResult("missing"); found {
		t.Fatal("missing series found")
	}

	episodes, found := catalog.Episodes(showID, nil)
	if !found {
		t.Fatal("series episodes not found")
	}
	assertResultCount(t, episodes, 2)
	episodes, _ = catalog.Episodes(showID, url.Values{"SeasonId": {SeasonID(showID, 1)}})
	assertResultCount(t, episodes, 1)
	episodes, _ = catalog.Episodes(showID, url.Values{"Season": {"2"}})
	assertResultCount(t, episodes, 1)
	if _, found = catalog.Episodes("missing", nil); found {
		t.Fatal("missing series episodes found")
	}
}

func TestCatalogFeedsAndQueryHelpers(t *testing.T) { //nolint:cyclop // Related feed and query boundaries share one fixture.
	t.Parallel()
	items := catalogFixture()
	catalog := fixtureCatalog(items)

	latest, err := catalog.Latest(url.Values{"Limit": {"1"}})
	if err != nil || len(latest) != 1 || latest[0]["Name"] != "Photo" {
		t.Fatalf("latest = %#v, %v", latest, err)
	}
	if _, err = catalog.Latest(url.Values{"ids": {"one", "two"}}); err == nil {
		t.Fatal("invalid latest ids succeeded")
	}

	resume := catalog.Resume(func(item library.Item) UserState {
		return UserState{Seconds: map[string]float64{"movie00000000001": 12, "episode00000001": 20}[item.ID], Watched: item.ID == "episode00000001"}
	})
	assertResultCount(t, resume, 1)

	next := catalog.NextUp(func(item library.Item) bool { return item.ID == "episode00000001" })
	assertResultCount(t, next, 1)

	values := url.Values{"LiMiT": {"7"}, "Empty": nil}
	if Query(values, "limit") != "7" || Query(values, "empty") != "" || Query(values, "missing") != "" {
		t.Fatalf("query mismatch: %q %q %q", Query(values, "limit"), Query(values, "empty"), Query(values, "missing"))
	}
	if Int(values, "limit") != 7 || !Includes(" Movie,EPISODE ", "episode") || Includes("Movie", "Series") {
		t.Fatal("query helpers mismatch")
	}
	if stringValue(42) != "" {
		t.Fatal("non-string value was projected")
	}
}

func TestDTOProjectionUsesPlayerContract(t *testing.T) { //nolint:cyclop,funlen,gocognit // One table checks all canonical item variants and optional fields.
	t.Parallel()
	movie := catalogFixture()[0]
	options := ItemOptions{CanDownload: true, UserData: map[string]any{"Played": true}, MediaSource: map[string]any{"Id": "source"}}
	dto := ItemDTO(movie, options)
	for key, want := range map[string]any{"Type": "Movie", "MediaType": "Video", "CanDownload": true, "ProductionYear": 2024} {
		if dto[key] != want {
			t.Fatalf("movie %s = %#v, want %#v", key, dto[key], want)
		}
	}
	if _, ok := dto["ImageTags"]; !ok {
		t.Fatal("movie image tag missing")
	}
	if !reflect.DeepEqual(dto["Genres"], []string{"Drama", "Mystery"}) {
		t.Fatalf("genres = %#v", dto["Genres"])
	}

	episode := catalogFixture()[1]
	episodeDTO := ItemDTO(episode, options)
	if episodeDTO["Type"] != "Episode" || episodeDTO["SeriesName"] != "Display Show" || episodeDTO["IndexNumber"] != 1 {
		t.Fatalf("episode = %#v", episodeDTO)
	}
	episode.ShowTitle, episode.Year, episode.Genres, episode.Artwork = "", "invalid", "", ""
	if fallback := ItemDTO(episode, options); fallback["SeriesName"] != "Raw Show" {
		t.Fatalf("fallback series = %#v", fallback["SeriesName"])
	}

	audio := ItemDTO(library.Item{ID: "audio0000000001", Kind: "audio"}, options)
	photo := ItemDTO(library.Item{ID: "photo0000000001", Kind: "photo"}, options)
	if audio["Type"] != "Audio" || audio["MediaType"] != "Audio" || photo["Type"] != "Photo" || photo["MediaType"] != "Photo" {
		t.Fatalf("media kinds = %#v %#v", audio, photo)
	}

	episode.Artwork = "episode.jpg"
	showWithArt := library.Show{ID: ShowID("Raw Show"), Title: "Display Show", ArtworkID: "episode00000001", Episodes: []library.Item{episode}}
	if _, ok := SeriesDTO(showWithArt)["ImageTags"]; !ok {
		t.Fatal("series image tag missing")
	}
	showWithoutArt := library.Show{ID: "show00000000001", Title: "No Art"}
	if _, ok := SeriesDTO(showWithoutArt)["ImageTags"]; ok {
		t.Fatal("unexpected series image tag")
	}
	if _, ok := SeasonDTO(showWithArt, 1)["ImageTags"]; !ok {
		t.Fatal("season image tag missing")
	}
	if _, ok := SeasonDTO(showWithoutArt, 2)["ImageTags"]; ok {
		t.Fatal("unexpected season image tag")
	}

	if ID("1234567890abcdef") != "1234567890abcdef0000000000000000" || RawID("1234567890abcdef0000000000000000") != "1234567890abcdef" || RawID("short") != "short" {
		t.Fatal("item identifier mismatch")
	}
	if ShowID("RAW SHOW") != ShowID("raw show") || SeasonID("show", 15) != "show000000000000000f" {
		t.Fatal("hierarchy identifier mismatch")
	}
	if !reflect.DeepEqual(Seasons(library.Show{Episodes: []library.Item{{Season: 2}, {Season: 1}, {Season: 2}}}), []int{1, 2}) {
		t.Fatal("seasons were not unique and sorted")
	}
	if FolderDTO("id", "Name", "kind")["CollectionType"] != "kind" {
		t.Fatal("folder collection mismatch")
	}
}

func catalogFixture() []library.Item {
	base := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	return []library.Item{
		{ID: "movie00000000001", Kind: "video", Title: "Movie", Year: "2024", Genres: "Drama · Mystery", Artwork: "movie.jpg", Container: "MP4", Added: base, ProviderIDs: map[string]string{"imdb": "tt1234567"}, Subtitles: []string{"movie.srt"}},
		{ID: "episode00000001", Kind: "video", Title: "Pilot", Show: "Raw Show", ShowTitle: "Display Show", Season: 1, Episode: 1, Artwork: "episode.jpg", ShowArtwork: "show.jpg", Added: base.Add(time.Hour)},
		{ID: "episode00000002", Kind: "video", Title: "Return", Show: "Raw Show", ShowTitle: "Display Show", Season: 2, Episode: 1, Added: base.Add(2 * time.Hour)},
		{ID: "photo0000000001", Kind: "photo", Title: "Photo", Added: base.Add(3 * time.Hour)},
	}
}

func fixtureCatalog(items []library.Item) Catalog { return NewCatalog(items, fixtureProject) }

func fixtureProject(item library.Item) map[string]any {
	return ItemDTO(item, ItemOptions{UserData: ProjectUserDataDTO(item, UserState{}, false, nil), MediaSource: map[string]any{"Id": item.ID}})
}

func assertResultCount(t *testing.T, result map[string]any, want int) {
	t.Helper()
	items, ok := result["Items"].([]map[string]any)
	if !ok || len(items) != want {
		t.Fatalf("items = %#v, want %d", result["Items"], want)
	}
}
