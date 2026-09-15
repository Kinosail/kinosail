package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSubtitleProviderRejectsMalformedSearchAndDownloads(t *testing.T) { //nolint:cyclop,funlen // Each title selects one provider failure while all requests stay on the public fetch seam.
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		query := request.URL.Query().Get("film_name")
		switch request.URL.Path {
		case "/subtitles":
			switch query {
			case "Empty":
				_, _ = writer.Write([]byte(`{"status":true,"subtitles":[]}`))
			case "NoFiles":
				_, _ = writer.Write([]byte(`{"status":false,"subtitles":[]}`))
			case "BadJSON":
				_, _ = writer.Write([]byte(`{`))
			case "Untrusted":
				_, _ = writer.Write([]byte(`{"status":true,"subtitles":[{"url":"http://example.com/steal"}]}`))
			default:
				_ = json.NewEncoder(writer).Encode(map[string]any{"status": true, "subtitles": []any{map[string]string{"url": "http://" + request.Host + "/invalid.srt"}}})
			}
		case "/invalid.srt":
			_, _ = writer.Write([]byte("not subtitles"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	for _, title := range []string{"Empty", "NoFiles", "BadJSON", "Untrusted", "Invalid"} {
		media := t.TempDir()
		if err := os.WriteFile(filepath.Join(media, title+".mp4"), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
		handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"}})
		id := firstWebItemID(t, handler)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", nil))
		if response.Code != http.StatusBadGateway {
			t.Fatalf("%s fetch = %d %q", title, response.Code, response.Body.String())
		}
	}
}

func TestExternalProviderActionsRejectInputBeforeNetworkContact(t *testing.T) {
	t.Parallel()
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	t.Cleanup(provider.Close)
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{
		MediaDir: media, CacheDir: t.TempDir(),
		Metadata:  server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"},
		Subtitles: server.SubtitleConfig{URL: provider.URL, APIKey: "key"},
	})
	id := firstWebItemID(t, handler)
	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh?extra=true", nil))
	subtitleRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/subtitles/"+id+"/fetch", strings.NewReader("extra=true"))
	subtitles := httptest.NewRecorder()
	handler.ServeHTTP(subtitles, subtitleRequest)
	if metadata.Code != http.StatusBadRequest || subtitles.Code != http.StatusBadRequest || requests != 0 {
		t.Fatalf("provider action validation = metadata %d, subtitles %d, network %d", metadata.Code, subtitles.Code, requests)
	}
}

func TestOIDCRejectsUnsafeOriginsAndInvalidProtocolTransitions(t *testing.T) {
	servertest.AssertExternalOIDCBoundaries(t, servertest.OIDCFixture{
		NewHandler: func(dataDir string, oidc server.OIDCConfig, scim server.SCIMConfig) http.Handler {
			return server.New(server.Config{DataDir: dataDir, RequireAuth: true, OIDC: oidc, SCIM: scim})
		},
	})
}

var firstWebItemID = servertest.ExternalBoundaryItemID
