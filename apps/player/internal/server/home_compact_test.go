package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestCompactHomeKeepsFeaturedResumeOutOfShelf(t *testing.T) {
	for _, scenario := range []struct {
		count    int
		backdrop bool
	}{{1, false}, {6, false}, {1, true}} {
		count := scenario.count
		t.Run(fmt.Sprintf("count=%d/backdrop=%t", count, scenario.backdrop), func(t *testing.T) {
			media := t.TempDir()
			for i := range count {
				name := filepath.Join(media, fmt.Sprintf("Movie %d", i))
				if err := os.WriteFile(name+".mp4", []byte("media"), 0o600); err != nil {
					t.Fatal(err)
				}
				if scenario.backdrop {
					writeFeaturedMoviePoster(t, name+"-fanart.png")
				} else {
					writeFeaturedMoviePoster(t, name+".png")
				}
			}
			handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
			get := func() string {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
				if response.Code != http.StatusOK {
					t.Fatalf("home status = %d", response.Code)
				}
				return response.Body.String()
			}
			seen := map[string]bool{}
			for _, match := range regexp.MustCompile(`/item/([a-f0-9]+)`).FindAllStringSubmatch(get(), -1) {
				id := match[1]
				if seen[id] {
					continue
				}
				seen[id] = true
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=60"))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code >= 400 {
					t.Fatalf("save progress: %d", response.Code)
				}
			}
			body := get()
			feature := regexp.MustCompile(`(?s)<section class="home-feature".*?</section>`).FindString(body)
			match := regexp.MustCompile(`data-palette-id="([a-f0-9]+)"`).FindStringSubmatch(feature)
			if len(match) != 2 {
				t.Fatal("missing featured resume")
			}
			id := match[1]
			for _, fragment := range []string{`href="/watch/` + id + `"`, `href="/item/` + id + `"`, `action="/continue-watching/` + id + `/remove"`, `data-watch-progress="` + id + `"`, "Resume at 1m"} {
				if !strings.Contains(feature, fragment) {
					t.Errorf("feature missing %s", fragment)
				}
			}
			artwork := []string{`data-artwork="poster"`, `src="/art/` + id + `"`, `width="400" height="600"`}
			if scenario.backdrop {
				artwork = []string{`data-artwork="backdrop"`, `src="/backdrop/` + id + `"`, `width="1600" height="900"`}
			}
			for _, fragment := range artwork {
				if !strings.Contains(feature, fragment) {
					t.Errorf("feature missing %s", fragment)
				}
			}
			shelf := regexp.MustCompile(`(?s)<section class="home-shelf continue-shelf">.*?</section>`).FindString(body)
			if count == 1 && shelf != "" {
				t.Fatal("only featured item must not leave an empty shelf")
			}
			if count > 1 && strings.Count(shelf, `class="card resume-card"`) != 4 {
				t.Fatal("shelf must retain four other titles")
			}
			if strings.Contains(shelf, `/watch/`+id) {
				t.Fatal("featured title repeated in resume shelf")
			}
			if strings.Contains(shelf, `/backdrop/`) {
				t.Fatal("resume poster must use portrait artwork")
			}
		})
	}
}
