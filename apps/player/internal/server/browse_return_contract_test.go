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

// The browser owns tab context. A Referer must not alter the Go fallback or
// canonical public watch URL; the rendered labels and marker enable that UI.
func TestPlayerBrowseReturnKeepsCanonicalWatchAndSafeFallback(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Fictional Return Movie.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media})
	browse := httptest.NewRecorder()
	handler.ServeHTTP(browse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&sort=title&limit=4", nil))
	watch := regexp.MustCompile(`href="(/watch/[a-f0-9]{16})"`).FindStringSubmatch(browse.Body.String())
	if browse.Code != http.StatusOK || len(watch) != 2 {
		t.Fatal("fictional canonical watch prerequisite failed")
	}
	for _, referer := range []string{"", "http://example.com/?view=movies", "https://untrusted.invalid/?q=private"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, watch[1], nil)
		request.Header.Set("Referer", referer)
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, request)
		if page.Code != http.StatusOK || page.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatal("Player or no-referrer contract failed")
		}
		for _, required := range []string{
			`class="back" href="/" data-browse-return`,
			`data-return-movies="Back to Movies"`,
			`data-return-shows="Back to Shows"`,
			`data-return-search="Back to search results"`,
			`data-viewer-profile="local-owner"`,
			`/static/main.kinosail.bundle.js?v=36-htmx4`,
		} {
			if !strings.Contains(page.Body.String(), required) {
				t.Errorf("Go Player does not expose %s", required)
			}
		}
		if strings.Contains(page.Body.String(), "untrusted.invalid") || page.Header().Get("Location") != "" {
			t.Fatal("Referer must not become return content or a redirect")
		}
	}
}
