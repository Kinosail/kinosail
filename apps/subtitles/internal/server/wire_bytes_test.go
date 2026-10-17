package server_test

import "testing"

func TestCatalogListingsCompressWithoutChangingMediaOrPages(t *testing.T) {
	assetContracts.CatalogListingsCompressWithoutChangingMediaOrPages(t, "/api/v1/library")
}

func TestArtworkRevalidatesInsteadOfRedownloading(t *testing.T) {
	assetContracts.ArtworkRevalidatesInsteadOfRedownloading(t, "/art/{movie}")
}
