package servertest

import (
	"encoding/json"
	"net/http"
	"testing"
)

type metadataProviderContract struct {
	newServer func(*testing.T, string, string) http.Handler
	idFirst   func(string, string, bool, *int, *int) http.HandlerFunc
}

// RunMetadataProvider runs every provider regression against the real app server.
func RunMetadataProvider(t *testing.T, newServer func(*testing.T, string, string) http.Handler, idFirst func(string, string, bool, *int, *int) http.HandlerFunc) {
	suite := metadataProviderContract{newServer, idFirst}
	t.Run("OwnerCanRefreshProviderMetadataAndArtwork", suite.OwnerCanRefreshProviderMetadataAndArtwork)
	t.Run("ProviderUsesShowPosterWhenEpisodeStillIsMissing", suite.ProviderUsesShowPosterWhenEpisodeStillIsMissing)
	t.Run("RefreshMissingRepairsExistingShowArtwork", suite.RefreshMissingRepairsExistingShowArtwork)
	t.Run("ProviderUsesFilenameIMDbIDBeforeTitleSearch", suite.ProviderUsesFilenameIMDbIDBeforeTitleSearch)
	t.Run("ProviderFallsBackToCleanTitleAndYearForMalformedIMDbID", suite.ProviderFallsBackToCleanTitleAndYearForMalformedIMDbID)
	t.Run("ProviderFallsBackToCleanTitleAndYearWhenIMDbHasNoMatch", suite.ProviderFallsBackToCleanTitleAndYearWhenIMDbHasNoMatch)
}

// MetadataProviderFixture serves movie metadata and artwork for server tests.
func MetadataProviderFixture(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/poster.jpg" && request.Header.Get("Authorization") != "Bearer token" {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch request.URL.Path {
	case "/search/movie":
		_ = json.NewEncoder(writer).Encode(map[string]any{"results": []any{map[string]any{"id": 101, "title": "Arrival", "overview": "A linguist meets visitors.", "release_date": "2016-11-11", "poster_path": "/poster.jpg"}}})
	case "/movie/101":
		_ = json.NewEncoder(writer).Encode(map[string]any{"belongs_to_collection": map[string]any{"name": "Arrival Collection"}})
	case "/poster.jpg":
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("poster"))
	default:
		http.NotFound(writer, request)
	}
}
