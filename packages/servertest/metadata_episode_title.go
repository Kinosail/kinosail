package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func (suite metadataProviderContract) ProviderKeepsFilenameEpisodeTitleWhenEpisodeIsMissing(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(metadataShowPosterHandler())
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	files := []string{filepath.Join("Severance", "Season 01", "Severance.S01E01.mkv"), filepath.Join("Severance", "Season 02", "Severance S02E01 Hello Ms Cobel.mkv")}
	for _, name := range files {
		path := filepath.Join(mediaDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	for _, item := range metadataLibraryItems(t, handler) {
		refresh := httptest.NewRecorder()
		handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+item.ID+"/metadata/refresh", nil))
		if refresh.Code != http.StatusNoContent {
			t.Fatalf("refresh %s = %d %q", item.Title, refresh.Code, refresh.Body.String())
		}
	}
	want := map[string]bool{"S01E01 · Good News About Hell": false, "S02E01 · Hello Ms Cobel": false}
	items := metadataLibraryItems(t, handler)
	for _, item := range items {
		if _, ok := want[item.Title]; ok && item.Show == "Severance" {
			want[item.Title] = true
		}
	}
	for title, found := range want {
		if !found {
			t.Errorf("episode title %q missing after provider refresh: %#v", title, items)
		}
	}
}

type metadataLibraryItem struct{ ID, Title, Show string }

func metadataLibraryItems(t *testing.T, handler http.Handler) []metadataLibraryItem {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var page struct{ Items []metadataLibraryItem }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 2 {
		t.Fatalf("library = %d %q", response.Code, response.Body.String())
	}
	return page.Items
}
