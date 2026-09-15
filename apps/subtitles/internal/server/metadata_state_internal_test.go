package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest"
)

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
