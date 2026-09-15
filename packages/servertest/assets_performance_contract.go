package servertest

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// VersionedApplicationStylesheet checks the exact app CSS derivative and immutable response.
func (fixture AssetsFixture) VersionedApplicationStylesheet(t *testing.T, target string, baseCSS []byte) {
	t.Helper()
	t.Parallel()
	supporter, err := os.ReadFile("static/supporter.css")
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	fixture.NewHandler("", "", false).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	want := append(append([]byte(nil), baseCSS...), supporter...)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !bytes.Equal(response.Body.Bytes(), want) {
		t.Fatalf("versioned application stylesheet = status %d, cache %q, bytes %d; want bytes %d", response.Code, response.Header().Get("Cache-Control"), response.Body.Len(), len(want))
	}
}
