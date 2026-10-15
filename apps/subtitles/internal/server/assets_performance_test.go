package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/webassets"
)

func TestVersionedStaticAssetsUseImmutableCaching(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	for target, expected := range map[string]string{
		"/static/theme.js":      "public, max-age=86400",
		"/static/theme.js?v=32": "public, max-age=31536000, immutable",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != expected {
			t.Fatalf("%s cache = %d %q", target, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}

func TestVersionedApplicationStylesheetIncludesSupporterStyles(t *testing.T) {
	t.Parallel()
	supporter, err := os.ReadFile("static/supporter.css")
	if err != nil {
		t.Fatal(err)
	}
	dashboard, err := os.ReadFile("static/subtitle-dashboard.css")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := os.ReadFile("static/subtitle-workspace.css")
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(append(append([]byte(nil), webassets.SubtitlesCSS...), webassets.LastLightCSS...), supporter...), append(dashboard, workspace...)...)
	response := httptest.NewRecorder()
	assetContracts.NewHandler("", "", false).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css?v=impeccable-1", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !bytes.Equal(response.Body.Bytes(), want) {
		t.Fatalf("versioned application stylesheet = status %d, cache %q, bytes %d; want bytes %d", response.Code, response.Header().Get("Cache-Control"), response.Body.Len(), len(want))
	}
}
