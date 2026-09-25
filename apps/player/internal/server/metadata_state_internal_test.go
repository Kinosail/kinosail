package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMetadataStoreLoadsPersistedMultilinePlots(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), []byte(`{"item":{"Title":"Movie","Plot":"First paragraph\nSecond paragraph","ShowPlot":"First line\r\n\tSecond line"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newMetadataStore(MetadataConfig{}, directory, "")
	if store.err != nil || store.records["item"].Plot != "First paragraph\nSecond paragraph" {
		t.Fatalf("persisted multiline metadata = %#v, %v", store.records["item"], store.err)
	}
}

func TestFailedBackdropDownloadLeavesRecordRetryable(t *testing.T) {
	provider := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(provider.Close)
	store := newMetadataStore(MetadataConfig{ImageURL: provider.URL}, "", t.TempDir())
	for _, test := range []struct {
		name, target string
		show         bool
	}{
		{"movie", "movie.jpg", false},
		{"show", "show.jpg", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := metadataRecord{Title: "Title", BackdropChecked: true}
			if test.show {
				record.ShowBackdrop = test.target
			} else {
				record.Backdrop = test.target
			}
			result := sharedmetadata.Result{Record: record, Images: []sharedmetadata.Image{{Source: "/missing.jpg", Target: test.target, Show: test.show, Backdrop: true}}}
			updated, err := store.downloadMetadataImages(t.Context(), result)
			if err == nil || updated.BackdropChecked || updated.Backdrop != "" || updated.ShowBackdrop != "" {
				t.Fatalf("failed backdrop = %#v, %v", updated, err)
			}
		})
	}
}

func TestMetadataStoreDetachesMutableRecords(t *testing.T) {
	t.Parallel()
	store := newMetadataStore(MetadataConfig{}, "", "")
	servertest.MetadataStoreDetachesMutableRecords(t,
		func(providerIDs, showProviderIDs map[string]string) error {
			return store.set("item", metadataRecord{Title: "Arrival", ProviderIDs: providerIDs, ShowProviderIDs: showProviderIDs})
		},
		func() map[string]string { return store.apply([]library.Item{{ID: "item"}})[0].ProviderIDs },
	)
}

func TestMetadataStoreAcceptsPersistedMultilinePlots(t *testing.T) {
	t.Parallel()
	databaseStore, err := database.Open(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = databaseStore.Close() })
	records := map[string]metadataRecord{"movie": {Title: "Movie", Plot: "First paragraph\nSecond paragraph", ShowPlot: "First line\r\nSecond line"}}
	if err := databaseStore.SaveJSON("metadata.json", records); err != nil {
		t.Fatal(err)
	}
	store := newMetadataStore(MetadataConfig{}, t.TempDir(), t.TempDir(), databaseStore)
	if store.err != nil {
		t.Fatalf("valid persisted multiline metadata was rejected: %v", store.err)
	}
}
