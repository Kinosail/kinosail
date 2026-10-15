package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var metadataContracts = servertest.MetadataFixture{
	New: func(config servertest.MetadataConfig) http.Handler {
		return server.New(server.Config{MediaDir: config.MediaDir, CacheDir: config.CacheDir, TMDBToken: config.TMDBToken, TMDBURL: config.TMDBURL, TMDBImageURL: config.TMDBImageURL})
	},
	IDFirst: idFirstTMDB,
}

func TestLegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch(t *testing.T) {
	metadataContracts.LegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch(t)
}
