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

func AssertPictureInPictureControl(t *testing.T, newServer func(string) http.Handler) {
	t.Parallel()
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Picture-in-Picture.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newServer(mediaDir)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if !strings.Contains(page.Body.String(), `data-player-pip`) || !strings.Contains(page.Body.String(), `class=i-pip`) {
		t.Fatalf("player lacks Picture-in-Picture control: %q", page.Body.String())
	}
}
