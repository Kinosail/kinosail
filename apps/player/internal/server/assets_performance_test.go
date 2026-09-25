package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/MikeO7/kinosail/packages/webassets"
)

func TestLibraryPrioritizesVisibleArtworkWithoutLayoutShift(t *testing.T) {
	assetContracts.LibraryPrioritizesVisibleArtworkWithoutLayoutShift(t)
}

func TestVersionedStaticAssetsUseImmutableCaching(t *testing.T) {
	assetContracts.VersionedStaticAssetsUseImmutableCaching(t)
}

func TestVersionedApplicationStylesheetIncludesSupporterStyles(t *testing.T) {
	t.Parallel()
	supporter, err := os.ReadFile("static/supporter.css")
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.ReadFile("static/home.css")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile("static/settings.css")
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(append(append(append([]byte(nil), webassets.PlayerCSS...), webassets.LastLightCSS...), supporter...), home...), settings...)
	response := httptest.NewRecorder()
	assetContracts.NewHandler("", "", false).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css?v=electric-27", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !bytes.Equal(response.Body.Bytes(), want) {
		t.Fatalf("versioned application stylesheet = status %d, cache %q, bytes %d; want bytes %d", response.Code, response.Header().Get("Cache-Control"), response.Body.Len(), len(want))
	}
}
