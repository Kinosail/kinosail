package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func TestViewerCanSeeLocalNfoMetadata(t *testing.T) {
	metadataContracts.ViewerCanSeeLocalNfoMetadata(t)
}

func TestEpisodeNfoKeepsSeasonAndEpisodeNumber(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	directory := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"Severance.S01E02.File.mkv": "video",
		"Severance.S01E02.File.nfo": `<episodedetails><title>Half Loop</title></episodedetails>`,
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	id := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+id, nil))

	if !strings.Contains(show.Body.String(), "S01E02 · Half Loop") {
		t.Fatalf("show = %q", show.Body.String())
	}
}

func TestViewerCanBrowseAndSearchRichLocalMetadata(t *testing.T) {
	metadataContracts.ViewerCanBrowseAndSearchRichLocalMetadata(t)
}

func TestViewerCanSeeTMDBMovieMetadataAndArtwork(t *testing.T) {
	metadataContracts.ViewerCanSeeTMDBMovieMetadataAndArtwork(t)
}

func TestLegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch(t *testing.T) {
	metadataContracts.LegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch(t)
}

func TestTMDBAttributionIsVisibleWhenEnabled(t *testing.T) {
	metadataContracts.TMDBAttributionIsVisibleWhenEnabled(t)
}
