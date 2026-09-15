package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestViewerContentRatingAppliesToBrowseAndMediaRoutes(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	for name, content := range map[string]string{
		"Alien.mp4": "adult video", "Alien.nfo": `<movie><title>Alien</title><mpaa>R</mpaa></movie>`,
		"Bluey.mp4": "family video", "Bluey.nfo": `<movie><title>Bluey</title><mpaa>PG</mpaa></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Kid&password=viewer-password&libraries=all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	kid := signInTestProfile(t, handler, "/login", "name=Kid&password=viewer-password")
	ownerHome := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=Alien", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(ownerHome, request)
	alienID := regexp.MustCompile(`/watch/([a-f0-9]+)"[^<]*<div class="poster"><i class=i-play aria-hidden=true></i></div><h2>Alien`).FindStringSubmatch(ownerHome.Body.String())[1]
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles/permissions", strings.NewReader("id="+storedProfileID(t, dataDir, "Kid")+"&rating=family&libraries=all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	kidHome := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(kid)
	handler.ServeHTTP(kidHome, request)

	for _, path := range []string{"/watch/", "/media/", "/hls/"} {
		response := httptest.NewRecorder()
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, path+alienID, nil)
		if path == "/hls/" {
			request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, path+alienID+"/index.m3u8", nil)
		}
		request.AddCookie(kid)
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("restricted %s = %d", path, response.Code)
		}
	}
	if strings.Contains(kidHome.Body.String(), "Alien") || !strings.Contains(kidHome.Body.String(), "Bluey") {
		t.Fatalf("restricted home = %q", kidHome.Body.String())
	}
}
