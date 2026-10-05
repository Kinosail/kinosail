package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
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
		// Explicit metadata keeps title bytes independent of filesystem normalization.
		title := strings.TrimSuffix(name, ".mp4")
		if err := os.WriteFile(filepath.Join(mediaDir, title+".nfo"), []byte("<movie><title>"+title+"</title></movie>"), 0o600); err != nil {
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
