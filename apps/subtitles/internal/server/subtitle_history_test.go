package server_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleHistoryShowsEmptyStateAndEarlierInstall(t *testing.T) {
	t.Parallel()
	media, dataDir := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: dataDir, CacheDir: t.TempDir()})
	empty := requestApp(t, handler, http.MethodGet, "/?view=history", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), "No subtitle changes yet.") || !strings.Contains(empty.Body.String(), "Find subtitles") {
		t.Fatalf("empty subtitle history = %d %q", empty.Code, empty.Body.String())
	}
	home := requestApp(t, handler, http.MethodGet, "/", "")
	match := regexp.MustCompile(`/subtitles/inspect/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("missing media identity: %q", home.Body.String())
	}
	now := time.Now().Unix()
	stored := fmt.Sprintf(`{"version":2,"records":{"%s:en":{"fingerprint":"%s","source":"subdl","score":90,"checked_at":%d,"installed_at":%d,"managed":true}}}`, match[1], strings.Repeat("0", 64), now, now)
	if err := os.WriteFile(filepath.Join(dataDir, "subtitle_acquisitions.json"), []byte(stored), 0o600); err != nil {
		t.Fatal(err)
	}
	reloaded := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: dataDir, CacheDir: t.TempDir()})
	page := requestApp(t, reloaded, http.MethodGet, "/?view=history", "")
	api := requestApp(t, reloaded, http.MethodGet, "/api/v1/subtitle-library?view=history", "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Arrival") || !strings.Contains(page.Body.String(), "SubDL") || !strings.Contains(page.Body.String(), "Added") || api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"matched":1`) || !strings.Contains(api.Body.String(), `"action":"added"`) {
		t.Fatalf("migrated subtitle history = %d %q; API = %d %q", page.Code, page.Body.String(), api.Code, api.Body.String())
	}
}
