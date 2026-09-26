package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleSettingsCleanupJourney(t *testing.T) { //nolint:cyclop // The public journey verifies preview, deletion, preferences, and preserved files together.
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt", "Film.es.srt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	if response := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es"]}`); response.Code != http.StatusOK {
		t.Fatalf("set languages: %d %s", response.Code, response.Body.String())
	}
	settings := requestApp(t, handler, http.MethodGet, "/settings", "")
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), "Delete subtitle languages") || !strings.Contains(settings.Body.String(), `name="forced"`) {
		t.Fatalf("cleanup setting: %d %s", settings.Code, settings.Body.String())
	}
	preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?language=en&forced=keep", "")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "1 subtitle file to delete") || !strings.Contains(preview.Body.String(), "Film.es.srt") || strings.Contains(preview.Body.String(), "<li><code>"+filepath.Join(media, "Film.en.forced.srt")) {
		t.Fatalf("cleanup preview: %d %s", preview.Code, preview.Body.String())
	}
	match := regexp.MustCompile(`name="digest" value="([0-9a-f]{64})"`).FindStringSubmatch(preview.Body.String())
	if len(match) != 2 {
		t.Fatal("preview digest missing")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/cleanup", strings.NewReader("language=en&forced=keep&digest="+match[1]))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Deleted 1 subtitle file") {
		t.Fatalf("delete: %d %s", result.Code, result.Body.String())
	}
	if _, err := os.Stat(filepath.Join(media, "Film.es.srt")); !os.IsNotExist(err) {
		t.Fatalf("Spanish subtitle remains: %v", err)
	}
	if settings := requestApp(t, handler, http.MethodGet, "/api/v1/settings", ""); !strings.Contains(settings.Body.String(), `"subtitleLanguages":["en"]`) {
		t.Fatalf("cleanup preferences: %d %s", settings.Code, settings.Body.String())
	}
	for _, name := range []string{"Film.en.srt", "Film.en.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("kept subtitle %s: %v", name, err)
		}
	}
}

func TestSubtitleCleanupCannotOverrideManagedLanguage(t *testing.T) {
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mkv"), "video")
	other := filepath.Join(media, "Film.es.srt")
	writeTestFile(t, other, "Spanish subtitles")
	configured, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		if key == "KINOSAIL_SUBTITLE_LANGUAGE" {
			return "es", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Configuration: configured})
	preview := requestApp(t, handler, http.MethodGet, "/settings/subtitles/cleanup?language=en&forced=keep", "")
	if preview.Code != http.StatusConflict {
		t.Fatalf("managed preview: %d %s", preview.Code, preview.Body.String())
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("managed cleanup changed media: %v", err)
	}
}
