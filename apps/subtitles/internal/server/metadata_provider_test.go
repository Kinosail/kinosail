package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMetadataProviderContract(t *testing.T) {
	servertest.RunMetadataProvider(t, func(t *testing.T, mediaDir, providerURL string) http.Handler {
		return server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: providerURL, ImageURL: providerURL, Token: "token"}})
	}, idFirstTMDB)
}

var fakeTMDB = servertest.MetadataProviderFixture
