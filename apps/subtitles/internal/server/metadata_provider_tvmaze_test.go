package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTVMazeEndpointIsPinnedToTrustedOrigin(t *testing.T) {
	for value, valid := range map[string]bool{
		"https://api.tvmaze.com":            true,
		"https://api.tvmaze.com/":           false,
		"https://evil.example":              false,
		"https://api.tvmaze.com?redirect=x": false,
		"https://user@api.tvmaze.com":       false,
		"http://127.0.0.1:8080":             true,
		"http://127.0.0.1":                  false,
	} {
		parsed, err := url.Parse(value)
		if err != nil || sharedmetadata.ValidTVMazeEndpoint(parsed) != valid {
			t.Fatalf("endpoint %q validity = %v, error = %v", value, sharedmetadata.ValidTVMazeEndpoint(parsed), err)
		}
	}
}

func TestTVMazeFallbackUsesTVDBIdentityAndEpisodeNumber(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(servertest.ServeTVMazeIdentityFixture))
	t.Cleanup(provider.Close)
	store := newMetadataStore(MetadataConfig{TVMazeURL: provider.URL}, "", "")
	result, err := store.resolveRecord(t.Context(), library.Item{Show: "Example", ShowTitle: "Example", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "123"}})
	if err != nil || result.Record.Title != "S01E02 · The Return" || result.Record.Plot != "A second chapter." || result.Record.Year != "2026" || result.Record.ShowPlot != "A useful show." || result.Record.ShowYear != "2025" || result.Record.ProviderIDs["tvdb"] != "123" || result.Record.ProviderIDs["tvmaze"] != "9001" || result.Record.ShowProviderIDs["tvmaze"] != "42" {
		t.Fatalf("TVmaze result = %#v, error = %v", result.Record, err)
	}
}

func TestTVMazeFallbackRejectsWrongEpisodeAndUnsafeRedirect(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/lookup/shows" && request.URL.Query().Get("thetvdb") == "123" {
			http.Redirect(writer, request, "https://evil.example/shows/42", http.StatusMovedPermanently)
			return
		}
		if request.URL.Path == "/lookup/shows" {
			http.Redirect(writer, request, "/shows/42", http.StatusMovedPermanently)
			return
		}
		_, _ = writer.Write([]byte(`{"id":9001,"name":"Wrong","season":9,"number":9,"airdate":"2026-01-02"}`))
	}))
	t.Cleanup(provider.Close)
	store := newMetadataStore(MetadataConfig{TVMazeURL: provider.URL}, "", "")
	_, err := store.resolveRecord(t.Context(), library.Item{Show: "Example", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "123"}})
	if err == nil || !strings.Contains(err.Error(), "metadata was not found") {
		t.Fatalf("unsafe TVmaze result error = %v", err)
	}
	_, err = store.resolveRecord(t.Context(), library.Item{Show: "Example", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "124"}})
	if err == nil || !strings.Contains(err.Error(), "metadata was not found") {
		t.Fatalf("wrong TVmaze episode error = %v", err)
	}
}
