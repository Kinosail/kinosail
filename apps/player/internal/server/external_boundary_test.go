package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestExternalMetadataActionRejectsInputBeforeNetworkContact(t *testing.T) {
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
		Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"},
	})
	id := firstWebItemID(t, handler)
	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh?extra=true", nil))
	if metadata.Code != http.StatusBadRequest || requests != 0 {
		t.Fatalf("metadata action validation = %d, network %d", metadata.Code, requests)
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
