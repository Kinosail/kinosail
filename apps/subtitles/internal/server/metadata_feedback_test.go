package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAutomaticMetadataDoesNotRetriggerUnchangedProviderResults(t *testing.T) {
	t.Parallel()
	servertest.RunAutomaticMetadataFeedback(t, func(ctx context.Context, media, cache, provider string) http.Handler {
		return server.New(server.Config{Lifecycle: ctx, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, Metadata: server.MetadataConfig{URL: provider, ImageURL: provider, Token: "token"}})
	})
}
