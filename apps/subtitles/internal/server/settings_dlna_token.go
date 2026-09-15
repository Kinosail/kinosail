package server

func (store *settingsStore) dlnaToken() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.value.DLNAToken
}
