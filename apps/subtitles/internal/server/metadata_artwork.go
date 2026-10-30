package server

import "github.com/MikeO7/kinosail/packages/metadata"

func (store *metadataStore) currentArtwork(path string) string {
	return store.artwork.Current(store.cache, path)
}

func (store *metadataStore) pruneExpiredArtwork() error {
	return metadata.PruneExpiredArtwork(store.cache)
}
