package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestWebProgressRejectsOversizedSession(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	id := firstAPIItemID(t, handler, owner.Value)
	body := url.Values{"seconds": {"42"}, "session": {strings.Repeat("s", 129)}, "revision": {"1"}}.Encode()
	response := requestWithCookie(t, handler, http.MethodPost, "/progress/"+id, body, owner)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized web progress session = %d %q", response.Code, response.Body.String())
	}
}
