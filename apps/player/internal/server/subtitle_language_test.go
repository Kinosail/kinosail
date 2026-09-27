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

func TestPreferredSubtitleLanguageUsesLocalTracksAcrossWebAndAPI(t *testing.T) { //nolint:cyclop // One lifecycle proves the shared local-only web and API contract.
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	for name, contents := range map[string]string{
		"Arrival.mp4":           "video",
		"Arrival.en.srt":        "1\n00:00:01,000 --> 00:00:02,000\nHello\n",
		"Arrival.fr.forced.srt": "1\n00:00:01,000 --> 00:00:02,000\nForced\n",
		"Arrival.fr.srt":        "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n",
	} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language=fr"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, request)
	id := firstWebItemID(t, handler)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	apiSettings := httptest.NewRecorder()
	handler.ServeHTTP(apiSettings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/settings", nil))

	if saved.Code != http.StatusSeeOther || !strings.Contains(settings.Body.String(), `name="language" value="fr"`) || !strings.Contains(settings.Body.String(), `href="https://github.com/Kinosail/kinosail/tree/main/apps/subtitles" rel="noreferrer">Kino Subtitles on GitHub</a>`) || strings.Contains(settings.Body.String(), "SubDL") {
		t.Fatalf("save = %d, settings = %q", saved.Code, settings.Body.String())
	}
	if !strings.Contains(player.Body.String(), `<track default kind="subtitles" label="French · Subtitles" data-subtitle-source="`) || !strings.Contains(playback.Body.String(), `"label":"French · Subtitles","source":"/subtitle/`+id+`/2","default":true,"language":"fr"`) {
		t.Fatalf("player = %q, playback = %q", player.Body.String(), playback.Body.String())
	}
	if strings.Contains(apiSettings.Body.String(), "subtitleProvider") {
		t.Fatalf("API settings still expose a subtitle provider: %q", apiSettings.Body.String())
	}
	for _, retired := range []struct{ method, path string }{
		{http.MethodPost, "/subtitles/" + id + "/fetch"},
		{http.MethodGet, "/subtitles/" + id + "/fr"},
		{http.MethodPost, "/api/v1/items/" + id + "/subtitles"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), retired.method, retired.path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("retired route %s %s = %d", retired.method, retired.path, response.Code)
		}
	}
}

func TestPreferredSubtitleLanguageRejectsInvalidValuesWithoutChangingSettings(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	for _, value := range []string{"", "z", "zz", "english", "e1"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles", strings.NewReader("language="+value))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid language %q = %d", value, response.Code)
		}
	}
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if match := regexp.MustCompile(`name="language" value="([^"]+)"`).FindStringSubmatch(settings.Body.String()); len(match) != 2 || match[1] != "en" {
		t.Fatalf("subtitle language changed after rejected input: %q", settings.Body.String())
	}
}
