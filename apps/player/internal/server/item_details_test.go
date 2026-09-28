package server_test

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestMovieCardsOpenPlayerAndOldDetailsRedirect(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	library := call(http.MethodGet, "/?view=movies", "")
	match := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(library.Body.String())
	if len(match) != 2 {
		t.Fatal("movie poster does not open playback")
	}
	path := "/item/" + match[1]
	player := call(http.MethodGet, "/watch/"+match[1], "")
	for _, expected := range []string{"<video", ">Add to My List</button>", "Playback &amp; downloads", "Movie"} {
		if player.Code != http.StatusOK || !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("player lacks %q", expected)
		}
	}
	response := call(http.MethodGet, path, "")
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/watch/"+match[1] {
		t.Fatalf("old details route = %d %q", response.Code, response.Header().Get("Location"))
	}
	assertMovieListReturnsToPlayer(t, call, path)
	assertInvalidMovieListActions(t, call, path)
	if call(http.MethodGet, path+"?unexpected=1", "").Code != http.StatusBadRequest {
		t.Fatal("ambiguous title query accepted")
	}
	if call(http.MethodGet, "/item/missing", "").Code != http.StatusNotFound {
		t.Fatal("missing title was not rejected")
	}
}

func TestMovieBrowseCardOmitsYearAndPlayerRetainsIt(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival (2016).mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
	get := func(path string) string {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d", path, response.Code)
		}
		return response.Body.String()
	}
	browse := get("/?view=movies")
	match := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(browse)
	if len(match) != 2 {
		t.Fatal("movie is missing from browse results")
	}
	if strings.Contains(browse, ">2016<") || strings.Contains(browse, " · 2016</h2>") {
		t.Fatal("browse card shows release year")
	}
	if !strings.Contains(get("/watch/"+match[1]), `<small>2016</small>`) {
		t.Fatal("player omits release year")
	}
}

func TestFeaturedMovieAndShelfOpenPlayer(t *testing.T) {
	media := t.TempDir()
	folder := filepath.Join(media, "Movie")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Movie.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeFeaturedMoviePoster(t, filepath.Join(folder, "Movie.png"))
	handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	feature := regexp.MustCompile(`(?s)<section class="home-feature".*?</section>`).FindString(body)
	play := regexp.MustCompile(`data-feature-action(?:="")? href="/watch/([a-f0-9]+)"`).FindStringSubmatch(feature)
	if len(play) != 2 {
		t.Fatal("featured movie lacks explicit playback")
	}
	if strings.Contains(feature, `href="/item/`+play[1]+`"`) || strings.Contains(feature, `View details`) {
		t.Fatal("featured movie still offers the removed detail screen")
	}
	shelf := regexp.MustCompile(`(?s)<section[^>]*data-home-shelf="recent-movies".*?</section>`).FindString(body)
	if !strings.Contains(shelf, `href="/watch/`+play[1]+`"`) {
		t.Fatal("browsing the movie does not open playback")
	}
	if strings.Contains(body, "<video") || strings.Contains(body, "autoplay") {
		t.Fatal("home must not start playback")
	}
}

func writeFeaturedMoviePoster(t *testing.T, path string) {
	t.Helper()
	poster, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(poster, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	if err := poster.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertInvalidMovieListActions(t *testing.T, call func(string, string, string) *httptest.ResponseRecorder, path string) {
	t.Helper()
	for _, body := range []string{"listed=maybe", "listed=true&listed=false", "listed=true&extra=1"} {
		rejected := call(http.MethodPost, path+"/list", body)
		if rejected.Code != http.StatusBadRequest {
			t.Fatalf("invalid list action accepted: %q", body)
		}
		if !strings.Contains(call(http.MethodGet, "/watch/"+strings.TrimPrefix(path, "/item/"), "").Body.String(), ">Add to My List") {
			t.Fatal("rejected list action changed state")
		}
	}
}

func assertMovieListReturnsToPlayer(t *testing.T, call func(string, string, string) *httptest.ResponseRecorder, path string) {
	t.Helper()
	for _, body := range []string{"listed=true", "listed=false"} {
		saved := call(http.MethodPost, path+"/list", body)
		if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/watch/"+strings.TrimPrefix(path, "/item/") {
			t.Fatalf("list action did not return to playback: %d %s", saved.Code, saved.Header().Get("Location"))
		}
	}
}
