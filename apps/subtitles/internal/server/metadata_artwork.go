package server

import "github.com/MikeO7/kinosail/packages/metadata"

const tmdbArtworkMaxAge = metadata.ArtworkMaxAge

func (store *metadataStore) currentArtwork(path string) string {
	return metadata.CurrentArtwork(store.cache, path)
}

func (store *metadataStore) pruneExpiredArtwork() error {
	return metadata.PruneExpiredArtwork(store.cache)
}
