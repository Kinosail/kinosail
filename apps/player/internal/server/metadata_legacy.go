package server

import sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"

func (store *metadataStore) migrateLegacyTMDB() {
	if store.err != nil {
		return
	}
	sharedmetadata.MigrateLegacyTMDB(store.cache, store.hasRecord, store.setAll, store.file != "")
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, record := range store.records {
		if record.ProviderIDs["tmdb"] != "" {
			store.tmdbRecords = true
			return
		}
	}
}

func (store *metadataStore) hasRecord(id string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	_, found := store.records[id]
	return found
}
