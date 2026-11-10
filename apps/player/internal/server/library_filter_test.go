package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type libraryFacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type filteredLibrary struct {
	Items   []struct{ ID, Title string } `json:"items"`
	Total   int                          `json:"total"`
	Filters struct {
		Genre, Decade, Rating, Watched string
	} `json:"filters"`
	Facets struct {
		Genres, Decades, Ratings []libraryFacetValue
	} `json:"facets"`
}

func libraryFilterHandler(t *testing.T) http.Handler {
	t.Helper()
	mediaDir := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		target := filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, nfo := range map[string]string{
		"Arrival":  "<year>2016</year><mpaa>PG-13</mpaa><genre>Drama</genre><genre>Science Fiction</genre>",
		"Contact":  "<year>1997</year><mpaa>PG</mpaa><genre>Drama</genre>",
		"Heat":     "<year>1995</year><mpaa>R</mpaa><genre>Crime</genre>",
		"Memento":  "<year>2000</year><mpaa>R</mpaa><genre>Crime</genre><genre>Drama</genre>",
		"Untagged": "",
	} {
		write(name+".mp4", "video")
		write(name+".nfo", "<movie><title>"+name+"</title>"+nfo+"</movie>")
	}
	write("Series (2022)/Season 01/Series - S01E01 - Pilot.mkv", "video")
	write("Series (2022)/Season 01/Series - S01E01 - Pilot.nfo", "<episodedetails><title>Pilot</title><genre>Drama</genre></episodedetails>")
	write("Series (2022)/Season 01/Series - S01E02 - Second.mkv", "video")
	write("Series (2022)/Season 01/Series - S01E02 - Second.nfo", "<episodedetails><title>Second</title><genre>Drama</genre></episodedetails>")
	return server.New(server.Config{MediaDir: mediaDir})
}

func filteredTitles(t *testing.T, handler http.Handler, query string) (filteredLibrary, []string) {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?"+query, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET library?%s = %d %q", query, response.Code, response.Body.String())
	}
	var page filteredLibrary
	mustJSON(t, response, &page)
	titles := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		titles = append(titles, item.Title)
	}
	return page, titles
}

func TestLibraryFiltersNarrowMoviesAndShowsThroughTheAPI(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	for query, expected := range map[string]string{
		"view=movies&genre=Drama":                          "Arrival,Contact,Memento",
		"view=movies&genre=Science+Fiction":                "Arrival",
		"view=movies&genre=Unknown":                        "",
		"view=movies&decade=1990":                          "Contact,Heat",
		"view=movies&decade=2000":                          "Memento",
		"view=movies&rating=R":                             "Heat,Memento",
		"view=movies&genre=Drama&rating=R":                 "Memento",
		"view=movies&genre=Crime&decade=1990&rating=R":     "Heat",
		"view=movies&genre=Drama&sort=year":                "Arrival,Memento,Contact",
		"view=movies&genre=Drama&letter=C":                 "Contact",
		"view=movies&genre=Drama&q=arrival":                "Arrival",
		"view=shows&genre=Drama":                           "Series",
		"view=shows&genre=Crime":                           "",
		"view=shows&decade=2020":                           "Series",
		"view=movies&genre=Drama&limit=1&offset=1":         "Contact",
		"view=movies&genre=Drama&limit=2&offset=0&lang=en": "Arrival,Contact",
	} {
		page, titles := filteredTitles(t, handler, query)
		if strings.Join(titles, ",") != expected {
			t.Errorf("library?%s = %v, want %s", query, titles, expected)
		}
		values, _ := url.ParseQuery(query)
		if page.Filters.Genre != values.Get("genre") || page.Filters.Decade != values.Get("decade") || page.Filters.Rating != values.Get("rating") {
			t.Errorf("library?%s filters = %+v", query, page.Filters)
		}
	}
	page, _ := filteredTitles(t, handler, "view=movies&genre=Drama&limit=1")
	if page.Total != 3 {
		t.Errorf("filtered total = %d, want 3", page.Total)
	}
}

func TestLibraryFacetsCountVisibleTitlesAndIgnoreTheActiveFilter(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	facet := func(values []libraryFacetValue) string {
		parts := make([]string, 0, len(values))
		for _, value := range values {
			parts = append(parts, value.Value+"="+strconv.Itoa(value.Count))
		}
		return strings.Join(parts, ",")
	}
	for _, query := range []string{"view=movies", "view=movies&genre=Crime&decade=1990", "view=movies&sort=added"} {
		page, _ := filteredTitles(t, handler, query)
		if genres := facet(page.Facets.Genres); genres != "Crime=2,Drama=3,Science Fiction=1" {
			t.Errorf("library?%s genres = %s", query, genres)
		}
		if decades := facet(page.Facets.Decades); decades != "2010=1,2000=1,1990=2" {
			t.Errorf("library?%s decades = %s", query, decades)
		}
		if ratings := facet(page.Facets.Ratings); ratings != "PG=1,PG-13=1,R=2" {
			t.Errorf("library?%s ratings = %s", query, ratings)
		}
	}
	shows, _ := filteredTitles(t, handler, "view=shows")
	if genres, decades := facet(shows.Facets.Genres), facet(shows.Facets.Decades); genres != "Drama=1" || decades != "2020=1" {
		t.Errorf("show facets = %s %s", genres, decades)
	}
}

func TestLibraryWatchedFilterFollowsTheProfileState(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	page, _ := filteredTitles(t, handler, "view=movies&q=Heat")
	if len(page.Items) != 1 {
		t.Fatalf("fixture = %+v", page.Items)
	}
	if response := apiCall(t, handler, "", http.MethodPut, "/api/v1/items/"+page.Items[0].ID+"/watched", map[string]any{"watched": true}); response.Code != http.StatusOK {
		t.Fatalf("mark watched = %d %q", response.Code, response.Body.String())
	}
	for query, expected := range map[string]string{
		"view=movies&watched=watched":                 "Heat",
		"view=movies&watched=unwatched":               "Arrival,Contact,Memento,Untagged",
		"view=movies&watched=unwatched&genre=Crime":   "Memento",
		"view=movies&watched=watched&genre=Drama":     "",
		"view=shows&watched=unwatched":                "Series",
		"view=shows&watched=watched":                  "",
		"view=movies&watched=watched&rating=R&q=heat": "Heat",
	} {
		page, titles := filteredTitles(t, handler, query)
		if strings.Join(titles, ",") != expected || page.Filters.Watched == "" {
			t.Errorf("library?%s = %v filters %+v, want %s", query, titles, page.Filters, expected)
		}
	}
}

func TestLibraryRejectsInvalidFiltersWithoutAResult(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	for _, query := range []string{
		"view=movies&decade=199", "view=movies&decade=1995", "view=movies&decade=abcd", "view=movies&decade=3000",
		"view=movies&watched=yes", "view=movies&genre=Drama&genre=Crime", "view=movies&genre=" + strings.Repeat("g", 65),
		"view=movies&rating=" + strings.Repeat("r", 17), "view=movies&genre=%ff", "view=movies&genre=Dra%00ma",
		"view=history&genre=Drama", "view=collections&decade=1990", "view=playlists&rating=R", "view=photos&watched=watched",
		"view=movies&genres=Drama",
	} {
		response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?"+query, nil)
		if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), `"items"`) {
			t.Errorf("library?%s = %d %q", query, response.Code, response.Body.String())
		}
		web := httptest.NewRecorder()
		handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?"+query, nil))
		if web.Code != http.StatusBadRequest {
			t.Errorf("web ?%s = %d", query, web.Code)
		}
	}
}

func TestLibraryPageOffersFiltersAndKeepsThemAcrossPages(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	get := func(path string) string {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
		return response.Body.String()
	}
	body := get("/?view=movies")
	for _, expected := range []string{`<select name="genre"`, `<option value="Science Fiction">Science Fiction (1)</option>`, `<select name="decade"`, `<option value="1990">1990s (2)</option>`, `<select name="rating"`, `<option value="PG-13">PG-13 (1)</option>`, `<select name="watched"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("movies page lacks %s", expected)
		}
	}
	if strings.Contains(body, "Clear filters") {
		t.Error("unfiltered page offers Clear filters")
	}
	body = get("/?view=movies&genre=Drama&limit=1")
	for _, expected := range []string{`<option value="Drama" selected>Drama (3)</option>`, `Clear filters`, `href="/?view=movies"`, ">Arrival</h2>", "genre=Drama"} {
		if !strings.Contains(body, expected) {
			t.Errorf("filtered page lacks %s", expected)
		}
	}
	if strings.Contains(body, ">Heat</h2>") || !strings.Contains(body, "<strong>3</strong>") {
		t.Error("filtered page shows an excluded movie or the wrong total")
	}
	if next := get("/?view=movies&genre=Drama&limit=1&offset=1"); !strings.Contains(next, ">Contact</h2>") || strings.Contains(next, ">Arrival</h2>") {
		t.Error("second filtered page does not continue the filtered list")
	}
	if empty := get("/?view=movies&genre=Unknown"); !strings.Contains(empty, "No titles match these filters.") || !strings.Contains(empty, "Clear filters") {
		t.Error("empty filtered page does not explain the empty result")
	}
	if photos := get("/?view=photos"); strings.Contains(photos, `name="genre"`) {
		t.Error("photos page offers title filters")
	}
}

func TestLibraryFiltersAndFacetsHideTitlesAboveTheViewerRating(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	for name, content := range map[string]string{
		"Alien.mp4": "adult video", "Alien.nfo": `<movie><title>Alien</title><year>1979</year><mpaa>R</mpaa><genre>Horror</genre></movie>`,
		"Bluey.mp4": "family video", "Bluey.nfo": `<movie><title>Bluey</title><year>2018</year><mpaa>PG</mpaa><genre>Family</genre></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	post := func(path, body string) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(owner)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	post("/settings/profiles", "name=Kid&password=viewer-password&libraries=all")
	kid := signInTestProfile(t, handler, "/login", "name=Kid&password=viewer-password")
	post("/settings/profiles/permissions", "id="+storedProfileID(t, dataDir, "Kid")+"&rating=family&libraries=all")
	get := func(cookie *http.Cookie, path string) (int, string) {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		request.AddCookie(cookie)
		handler.ServeHTTP(response, request)
		return response.Code, response.Body.String()
	}
	if status, body := get(owner, "/?view=movies&rating=R"); status != http.StatusOK || !strings.Contains(body, ">Alien</h2>") || !strings.Contains(body, `>Horror (1)</option>`) {
		t.Fatalf("Owner filter = %d, Alien shown = %t", status, strings.Contains(body, ">Alien</h2>"))
	}
	for _, path := range []string{"/?view=movies", "/?view=movies&sort=added", "/?view=movies&rating=R", "/?view=movies&genre=Horror", "/?view=movies&decade=1970", "/?view=movies&watched=unwatched&rating=R"} {
		status, body := get(kid, path)
		if status != http.StatusOK {
			t.Errorf("Viewer GET %s = %d", path, status)
		}
		for _, hidden := range []string{"Alien", ">Horror (", `value="1970"`, `<option value="R"`} {
			if strings.Contains(body, hidden) {
				t.Errorf("Viewer GET %s reveals %s", path, hidden)
			}
		}
	}
	if status, body := get(kid, "/?view=movies&genre=Family"); status != http.StatusOK || !strings.Contains(body, ">Bluey</h2>") {
		t.Errorf("Viewer allowed filter = %d", status)
	}
}
