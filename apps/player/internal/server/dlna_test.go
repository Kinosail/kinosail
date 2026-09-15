package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOwnerCanEnableBrowseAndRevokeDLNA(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, DLNAURL: "http://192.168.1.2:8080"})
	enable := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/dlna", strings.NewReader("enabled=true"))
	enable.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), enable)
	data := storedState(t, dataDir, "settings.json")
	var settings map[string]any
	if json.Unmarshal(data, &settings) != nil {
		t.Fatalf("DLNA token not persisted: %q", data)
	}
	token, ok := settings["dlnaToken"].(string)
	if !ok || token == "" {
		t.Fatalf("DLNA token not persisted: %q", data)
	}
	device := httptest.NewRecorder()
	handler.ServeHTTP(device, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dlna/"+token+"/device.xml", nil))
	mustDLNAResponse(t, device, http.StatusOK, "MediaServer:1", "/dlna/"+token+"/content")
	browse := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/dlna/"+token+"/content", strings.NewReader(`<s:Envelope><s:Body><u:Browse></u:Browse></s:Body></s:Envelope>`))
	browse.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, browse)
	mediaPath := regexp.MustCompile(`/dlna/` + token + `/media/([a-f0-9]+)`).FindString(result.Body.String())
	mustDLNAResponse(t, result, http.StatusOK, "Movie", mediaPath)
	media := httptest.NewRecorder()
	handler.ServeHTTP(media, httptest.NewRequestWithContext(t.Context(), http.MethodGet, mediaPath, nil))
	mustDLNAResponse(t, media, http.StatusOK, "movie")
	assertDLNASymlinkBlocked(t, handler, filepath.Join(mediaDir, "Movie.mp4"), mediaPath)
	disable := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/dlna", strings.NewReader("enabled=false"))
	disable.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), disable)
	revoked := httptest.NewRecorder()
	handler.ServeHTTP(revoked, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dlna/"+token+"/device.xml", nil))
	if revoked.Code != http.StatusNotFound {
		t.Fatalf("revoked device = %d %q", revoked.Code, revoked.Body.String())
	}
}

func assertDLNASymlinkBlocked(t *testing.T, handler http.Handler, mediaFile, mediaPath string) {
	t.Helper()
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(mediaFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, mediaFile); err != nil {
		t.Fatal(err)
	}
	escaped := httptest.NewRecorder()
	handler.ServeHTTP(escaped, httptest.NewRequestWithContext(t.Context(), http.MethodGet, mediaPath, nil))
	if escaped.Code != http.StatusNotFound {
		t.Fatalf("symlinked DLNA media = %d %q", escaped.Code, escaped.Body.String())
	}
}

func mustDLNAResponse(t *testing.T, response *httptest.ResponseRecorder, status int, values ...string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	for _, value := range values {
		if value == "" || !strings.Contains(response.Body.String(), value) {
			t.Fatalf("response missing %q: %q", value, response.Body.String())
		}
	}
}
