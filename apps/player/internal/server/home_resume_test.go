package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestViewerCanFindContinuedMedia(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=60"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	homeResponse = httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !strings.Contains(homeResponse.Body.String(), "Continue watching") || !strings.Contains(homeResponse.Body.String(), "Resume at 1m") {
		t.Fatalf("home = %q", homeResponse.Body.String())
	}
	if strings.Contains(homeResponse.Body.String(), `class="home-feature"`) {
		t.Fatal("resume content must not be duplicated in a featured banner")
	}
	continued := regexp.MustCompile(`(?s)<section class="home-shelf continue-shelf">(.*?)</section>`).FindString(homeResponse.Body.String())
	for _, expected := range []string{`<a href="/?view=history">See all</a>`, `<span data-watch-remaining aria-hidden="true"></span>`, `class="resume-link" href="/watch/` + id + `"`, `<h3>Heat</h3>`, `class="watch-progress" data-watch-progress="` + id + `" hidden`, `aria-label="Watch progress"`, `class="resume-action"`, `class="resume-action" aria-label="Resume"`, `action="/continue-watching/` + id + `/remove"`} {
		if !strings.Contains(continued, expected) {
			t.Errorf("continued title is missing %q: %s", expected, continued)
		}
	}
	if count := strings.Count(continued, `href="/watch/`+id+`"`); count != 1 {
		t.Errorf("continued title has %d playback links, want one", count)
	}
}
