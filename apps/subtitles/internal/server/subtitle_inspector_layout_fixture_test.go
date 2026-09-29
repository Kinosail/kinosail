package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUIStateFixturesSubtitleInspector(t *testing.T) { //nolint:paralleltest // This opt-in test writes a shared browser fixture directory.
	dir := os.Getenv("KINOSAIL_UI_FIXTURE_DIR")
	if dir == "" {
		t.Skip("KINOSAIL_UI_FIXTURE_DIR is not set")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // The caller explicitly supplies this test-artifact directory, as in TestWriteUIStateFixtures.
		t.Fatal(err)
	}
	handler, base, _ := subtitleInspectorFixture(t, "1\n00:00:01,000 --> 00:00:03,000\nHello world.\n\n2\n00:00:04,000 --> 00:00:06,000\nA second line.\n")
	id := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	for name, path := range map[string]string{
		"subtitle-inspector.html": "/subtitles/inspect/" + id,
		"subtitle-inspector.json": base + "/inspect?language=en",
		"app.css":                 "/static/app.css",
		"subtitle-inspector.css":  "/static/subtitle-inspector.css",
		"subtitle-inspector.js":   "/static/subtitle-inspector.js",
		"icon.svg":                "/static/icon.svg",
		"manrope.woff2":           "/static/manrope.woff2",
		"cinema-backdrop.jpg":     "/static/cinema-backdrop.jpg",
	} {
		response := requestApp(t, handler, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, response.Code, response.Body.String())
		}
		if err := os.WriteFile(filepath.Join(dir, name), response.Body.Bytes(), 0o600); err != nil { //nolint:gosec // Fixed filenames in the caller's test-artifact directory.
			t.Fatal(err)
		}
	}
}
