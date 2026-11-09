package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type nfoShow struct {
	ID, Title, Year, Plot, Genres, Studio string
	Episodes                              []struct{ Title, Rating string }
}

func writeShowFixture(t *testing.T, mediaDir string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		target := filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

var tvshowNFOFixture = map[string]string{
	"Series/tvshow.nfo":                                  `<tvshow><title> The Series </title><year>2019</year><plot> A local show plot. </plot><mpaa>TV-14</mpaa><genre>Drama</genre><genre> Mystery </genre><studio>Studio One</studio><uniqueid type="tmdb">4242</uniqueid></tvshow>`,
	"Series/Season 01/Series - S01E01 - Pilot.mkv":       "video",
	"Series/Season 01/Series - S01E02 - Second.mkv":      "video",
	"Series/Season 01/Series - S01E02 - Second.nfo":      `<episodedetails><title>Second</title><mpaa>TV-MA</mpaa></episodedetails>`,
	"Plain Show/Season 01/Plain Show - S01E01 - A.mkv":   "video",
	"Broken Show/tvshow.nfo":                             `<tvshow><title>Never closed`,
	"Broken Show/Season 01/Broken Show - S01E01 - A.mkv": "video",
	"Empty Show/tvshow.nfo":                              `<tvshow></tvshow>`,
	"Empty Show/Season 01/Empty Show - S01E01 - A.mkv":   "video",
	"Big Show/tvshow.nfo":                                `<tvshow><plot>` + strings.Repeat("x", 1<<20) + `</plot><title>Too Big</title></tvshow>`,
	"Big Show/Season 01/Big Show - S01E01 - A.mkv":       "video",
	"Film (2001).mp4":                                    "video",
	"tvshow.nfo":                                         `<tvshow><title>Root</title><mpaa>G</mpaa><genre>Wrong</genre></tvshow>`,
}

func showsByTitle(t *testing.T, handler http.Handler, cookie *http.Cookie) map[string]nfoShow {
	t.Helper()
	call := func(path string, target any) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
		mustJSON(t, response, target)
	}
	var listing struct{ Shows []struct{ ID, Title string } }
	call("/api/v1/shows", &listing)
	shows := map[string]nfoShow{}
	for _, entry := range listing.Shows {
		var show nfoShow
		call("/api/v1/shows/"+entry.ID, &show)
		if show.Title != entry.Title {
			t.Errorf("show list title %q differs from detail title %q", entry.Title, show.Title)
		}
		shows[show.Title] = show
	}
	return shows
}

func TestShowMetadataComesFromTVShowNFO(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	writeShowFixture(t, mediaDir, tvshowNFOFixture)
	shows := showsByTitle(t, server.New(server.Config{MediaDir: mediaDir}), nil)
	series, found := shows["The Series"]
	if !found || series.Year != "2019" || series.Plot != "A local show plot." || series.Genres != "Drama · Mystery" || series.Studio != "Studio One" {
		t.Fatalf("show from tvshow.nfo = %+v, titles = %v", series, shows)
	}
	if len(series.Episodes) != 2 || series.Episodes[0].Rating != "TV-14" || series.Episodes[1].Rating != "TV-MA" {
		t.Fatalf("episode ratings = %+v", series.Episodes)
	}
	for _, title := range []string{"Plain Show", "Broken Show", "Empty Show", "Big Show"} {
		show, found := shows[title]
		if !found || show.Year != "" || show.Plot != "" || show.Genres != "" || show.Studio != "" || len(show.Episodes) != 1 || show.Episodes[0].Rating != "" {
			t.Errorf("show %q without usable tvshow.nfo = %+v (found %t)", title, show, found)
		}
	}
	for _, unexpected := range []string{"Series", "Root", "Too Big", "Never closed"} {
		if _, found := shows[unexpected]; found {
			t.Errorf("unexpected show title %q", unexpected)
		}
	}
	if len(shows) != 5 {
		t.Errorf("shows = %d, want 5", len(shows))
	}
}

func TestShowRatingFromTVShowNFOAppliesTheViewerCeiling(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	writeShowFixture(t, mediaDir, tvshowNFOFixture)
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	post := func(path, body string) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(owner)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	post("/settings/profiles", "name=Teen&password=viewer-password&libraries=all")
	post("/settings/profiles", "name=Kid&password=viewer-password&libraries=all")
	post("/settings/profiles/permissions", "id="+storedProfileID(t, dataDir, "Teen")+"&rating=teen&libraries=all")
	post("/settings/profiles/permissions", "id="+storedProfileID(t, dataDir, "Kid")+"&rating=family&libraries=all")
	if shows := showsByTitle(t, handler, owner); len(shows) != 5 || len(shows["The Series"].Episodes) != 2 {
		t.Fatalf("Owner shows = %+v", shows)
	}
	teen := showsByTitle(t, handler, signInTestProfile(t, handler, "/login", "name=Teen&password=viewer-password"))
	if len(teen) != 1 || len(teen["The Series"].Episodes) != 1 || teen["The Series"].Episodes[0].Rating != "TV-14" {
		t.Fatalf("teen Viewer shows = %+v", teen)
	}
	if kid := showsByTitle(t, handler, signInTestProfile(t, handler, "/login", "name=Kid&password=viewer-password")); len(kid) != 0 {
		t.Fatalf("family Viewer shows = %+v", kid)
	}
}
