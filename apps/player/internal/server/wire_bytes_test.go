package server_test

import "testing"

func TestCatalogListingsCompressWithoutChangingMediaOrPages(t *testing.T) {
	assetContracts.CatalogListingsCompressWithoutChangingMediaOrPages(t, "/api/v1/library", "/api/v1/shows", "/api/v1/shows/{show}", "/api/v1/albums", "/api/v1/albums/{album}")
}
