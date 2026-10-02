package servertest

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// PlayerScriptURLTracksServedContents prevents immutable browser caches from retaining old playback code.
func (fixture AssetsFixture) PlayerScriptURLTracksServedContents(t *testing.T) {
	t.Helper()
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	item := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if home.Code != http.StatusOK || len(item) != 2 {
		t.Fatalf("library did not expose the test video: status %d", home.Code)
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+item[1], nil))
	script := regexp.MustCompile(`src="(/static/player\.js\?v=[^"]+)"`).FindStringSubmatch(page.Body.String())
	if page.Code != http.StatusOK || len(script) != 2 {
		t.Fatalf("watch page did not expose a versioned player script: status %d", page.Code)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, script[1], nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || response.Body.Len() == 0 {
		t.Fatalf("player script = status %d, cache %q, bytes %d", response.Code, response.Header().Get("Cache-Control"), response.Body.Len())
	}
	want := fmt.Sprintf("/static/player.js?v=%x", sha256.Sum256(response.Body.Bytes()))
	if script[1] != want {
		t.Fatalf("player script URL = %q; want served content hash URL %q", script[1], want)
	}
}

// LibraryPrioritizesVisibleArtworkWithoutLayoutShift preserves the shared app's asset performance contract.
func (fixture AssetsFixture) LibraryPrioritizesVisibleArtworkWithoutLayoutShift(t *testing.T) {
	t.Helper()
	t.Parallel()
	media := t.TempDir()
	for position := range 8 {
		name := filepath.Join(media, fmt.Sprintf("Movie %02d.mp4", position))
		if err := os.WriteFile(name, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
		name = filepath.Join(media, fmt.Sprintf("Movie %02d.jpg", position))
		if err := os.WriteFile(name, []byte("artwork"), 0o600); err != nil {
			t.Fatal(err)
		}
		name = filepath.Join(media, fmt.Sprintf("Photo %02d.jpg", position))
		if err := os.WriteFile(name, []byte("photo"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(media, "", false)
	for view, expectedDimensions := range map[string]string{"movies": `width="400" height="600"`, "photos": `width="400" height="300"`} {
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view="+view, nil))
		body := page.Body.String()
		if !strings.Contains(body, expectedDimensions+` decoding="async" fetchpriority="high"`) || !strings.Contains(body, expectedDimensions+` decoding="async" loading="lazy"`) {
			t.Fatalf("%s artwork loading contract missing: %q", view, body)
		}
		if strings.Count(body, `fetchpriority="high"`) != 2 {
			t.Fatalf("%s high-priority artwork = %d", view, strings.Count(body, `fetchpriority="high"`))
		}
	}
}

// VersionedStaticAssetsUseImmutableCaching preserves the shared app's asset performance contract.
func (fixture AssetsFixture) VersionedStaticAssetsUseImmutableCaching(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.NewHandler("", "", false)
	for target, expected := range map[string]string{
		"/static/player.js":      "public, max-age=86400",
		"/static/player.js?v=32": "public, max-age=31536000, immutable",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != expected {
			t.Fatalf("%s cache = %d %q", target, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}
