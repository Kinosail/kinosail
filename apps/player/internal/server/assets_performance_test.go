package server_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestLibraryPrioritizesVisibleArtworkWithoutLayoutShift(t *testing.T) {
	assetContracts.LibraryPrioritizesVisibleArtworkWithoutLayoutShift(t)
}

func TestVersionedStaticAssetsUseImmutableCaching(t *testing.T) {
	assetContracts.VersionedStaticAssetsUseImmutableCaching(t)
}

func TestPlayerScriptURLTracksServedContents(t *testing.T) {
	assetContracts.PlayerScriptURLTracksServedContents(t)
}

func TestHLSScriptReferencesTrackServedContents(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	fetch := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d", path, response.Code)
		}
		return response
	}
	want := fmt.Sprintf("/static/hls.min.js?v=%x", sha256.Sum256(fetch("/static/hls.min.js").Body.Bytes()))
	for _, path := range []string{"/static/player.js", "/static/theme.js"} {
		body := fetch(path).Body.String()
		if !strings.Contains(body, want) || regexp.MustCompile(`hls\.min\.js\?v=[0-9]+\.[0-9]`).MatchString(body) {
			t.Fatalf("%s does not reference the served hls.js content hash %q", path, want)
		}
	}
	if cache := fetch(want).Header().Get("Cache-Control"); cache != "public, max-age=31536000, immutable" {
		t.Fatalf("hashed hls.js cache = %q", cache)
	}
}

func TestStaticAssetsRevalidateWithETag(t *testing.T) {
	t.Parallel()
	handler := assetContracts.NewHandler("", "", false)
	for _, path := range []string{"/static/app.css", "/static/cinema-backdrop.jpg", "/manifest.webmanifest"} {
		for _, encoding := range []string{"", "gzip"} {
			first := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			request.Header.Set("Accept-Encoding", encoding)
			handler.ServeHTTP(first, request)
			etag := first.Header().Get("ETag")
			if first.Code != http.StatusOK || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
				t.Fatalf("%s (%q) = %d etag %q", path, encoding, first.Code, etag)
			}
			revalidate := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			revalidate.Header.Set("Accept-Encoding", encoding)
			revalidate.Header.Set("If-None-Match", `"stale", `+etag)
			second := httptest.NewRecorder()
			handler.ServeHTTP(second, revalidate)
			if second.Code != http.StatusNotModified || second.Body.Len() != 0 || second.Header().Get("ETag") != etag || second.Header().Get("Cache-Control") != first.Header().Get("Cache-Control") {
				t.Fatalf("%s (%q) revalidation = %d body %d etag %q cache %q", path, encoding, second.Code, second.Body.Len(), second.Header().Get("ETag"), second.Header().Get("Cache-Control"))
			}
			mismatch := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			mismatch.Header.Set("Accept-Encoding", encoding)
			mismatch.Header.Set("If-None-Match", `"stale"`)
			third := httptest.NewRecorder()
			handler.ServeHTTP(third, mismatch)
			if third.Code != http.StatusOK || third.Body.Len() != first.Body.Len() {
				t.Fatalf("%s (%q) stale validator = %d body %d", path, encoding, third.Code, third.Body.Len())
			}
		}
	}
}
