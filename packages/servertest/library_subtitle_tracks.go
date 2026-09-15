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

// ViewerCanChooseMultipleSubtitleTracks verifies the shared external-subtitle contract.
func (fixture LibraryAPIFixture) ViewerCanChooseMultipleSubtitleTracks(t *testing.T) {
	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Film.mp4":    "video",
		"Film.en.srt": "1\n00:00:00,000 --> 00:00:01,000\nHello\n",
		"Film.es.vtt": "WEBVTT\n\n00:00.000 --> 00:01.000\nHola\n",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	spanish := httptest.NewRecorder()
	handler.ServeHTTP(spanish, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/"+id+"/1", nil))
	if strings.Count(player.Body.String(), "<track ") != 2 || !strings.Contains(player.Body.String(), `label="EN"`) || !strings.Contains(player.Body.String(), `label="ES"`) || !strings.Contains(spanish.Body.String(), "Hola") {
		t.Fatalf("player = %q, spanish = %q", player.Body.String(), spanish.Body.String())
	}
}
