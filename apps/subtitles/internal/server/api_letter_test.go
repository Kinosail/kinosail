package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestNumericTitleJumpIsSharedByAPIAndWeb(t *testing.T) { //nolint:cyclop // One fixture compares the API and web projections.
	mediaDir := t.TempDir()
	for _, name := range []string{"2001 A Space Odyssey.mp4", "Alpha.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&letter=%23", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"letter":"#"`) || !strings.Contains(response.Body.String(), `"title":"2001 A Space Odyssey"`) || strings.Contains(response.Body.String(), `"title":"Alpha"`) {
		t.Fatalf("numeric letter API = %d %q", response.Code, response.Body.String())
	}
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&letter=%23", nil))
	body := web.Body.String()
	if web.Code != http.StatusOK || !strings.Contains(body, `aria-current="true">#</a>`) || !strings.Contains(body, ">2001 A Space Odyssey</h2>") || strings.Contains(body, ">Alpha</h2>") {
		t.Fatalf("numeric letter web = %d %q", web.Code, body)
	}
}

func TestLetterJumpUsesTheActiveLocaleForAccentedTitles(t *testing.T) { //nolint:cyclop // Locale-sensitive buckets are asserted together.
	mediaDir := t.TempDir()
	for _, name := range []string{"Alpha.mp4", "Ångström.mp4", "Beta.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&lang=en&letter=a", nil)
	var page struct {
		Items   []struct{ Title string } `json:"items"`
		Letter  string                   `json:"letter"`
		Letters []struct {
			Label string `json:"label"`
			Count int    `json:"count"`
		} `json:"letters"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || page.Letter != "A" || len(page.Items) != 2 {
		t.Fatalf("accented letter API = %#v, status = %d, error = %v, body = %q", page, response.Code, err, response.Body.String())
	}
	if page.Items[0].Title != "Alpha" || page.Items[1].Title != "Ångström" || len(page.Letters) != 2 || page.Letters[0].Label != "A" || page.Letters[0].Count != 2 || page.Letters[1].Label != "B" {
		t.Fatalf("accented letter page = %#v", page)
	}
}

func TestLetterJumpWebSupportsProgressiveMobileNavigation(t *testing.T) {
	handler := paginatedLibrary(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`type="search"`,
		`enterkeyhint="search"`,
		`autocapitalize="none"`,
		`data-title-jump data-letter-count="3"`,
		`data-title-jump-open aria-haspopup="dialog"`,
		`data-title-jump-index`,
		`data-title-jump-dialog aria-labelledby="title-jump-title"`,
		`data-title-letter="A" data-letter-count="1"`,
		`aria-label="A, 1 title"`,
		`hx-get="/?letter=A&amp;view=movies"`,
		`hx-swap="outerHTML show:top"`,
		`data-title-jump-preview`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("mobile title jump missing %q in %d %q", expected, response.Code, body)
		}
	}
}

func TestSingleTitleBucketDoesNotRenderAJumpControl(t *testing.T) {
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Alpha.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `data-title-jump`) {
		t.Fatalf("single title bucket = %d %q", response.Code, response.Body.String())
	}
}
