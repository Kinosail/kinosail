package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// LeanBackPlaybackDefaults verifies defaults, localized fields, overrides, and persistence.
func (fixture LibraryAPIFixture) LeanBackPlaybackDefaults(t *testing.T) { //nolint:cyclop // One lifecycle proves defaults, overrides, and restart persistence.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Severance.S01E01.Good.News.mkv", "Severance.S01E02.Half.Loop.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, dataDir, false)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))
	episodes := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindAllStringSubmatch(show.Body.String(), -1)
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+episodes[0][1], nil))

	for _, expected := range []string{`name="markers" value="intro" checked`, `name="markers" value="recap" checked`, `name="markers" value="commercial" checked`, `name="markers" value="outro" checked`, `name="markers" value="credits" checked`, `name="autoplay" value="true" checked`} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("default settings lack %q: %q", expected, settings.Body.String())
		}
	}
	spanish := httptest.NewRecorder()
	handler.ServeHTTP(spanish, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings?lang=es", nil))
	if !strings.Contains(spanish.Body.String(), `name="markers" value="intro" checked`) || strings.Contains(spanish.Body.String(), `name="autoOmitir"`) {
		t.Fatalf("localized playback fields = %q", spanish.Body.String())
	}
	if !strings.Contains(player.Body.String(), `data-auto-skip=""`) || !strings.Contains(player.Body.String(), `data-next="/watch/`+episodes[len(episodes)-1][1]+`"`) {
		t.Fatalf("default player = %q", player.Body.String())
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=automatic&subtitles=on"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	handler = fixture.NewHandler(mediaDir, dataDir, false)
	settings = httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	player = httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+episodes[0][1], nil))

	if strings.Contains(settings.Body.String(), `name="markers" value="intro" checked`) || strings.Contains(settings.Body.String(), `name="markers" value="credits" checked`) || strings.Contains(settings.Body.String(), `name="autoplay" value="true" checked`) || !strings.Contains(player.Body.String(), `data-auto-skip=""`) || strings.Contains(player.Body.String(), `data-next=`) {
		t.Fatalf("disabled settings = %q, player = %q", settings.Body.String(), player.Body.String())
	}
}
