package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTMDBArtworkExpiresAndIsNotApplied(t *testing.T) {
	servertest.ArtworkExpiresAndIsNotApplied(t, tmdbArtworkMaxAge, func(cache, path string) servertest.ArtworkExpiryFixture {
		store := newMetadataStore(MetadataConfig{}, "", cache)
		store.records["item"] = metadataRecord{Artwork: path, ShowArtwork: path}
		return servertest.ArtworkExpiryFixture{Apply: store.apply, Prune: store.pruneExpiredArtwork}
	})
}
