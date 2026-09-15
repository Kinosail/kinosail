package server

import sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"

func (store *settingsStore) ensureJellyfinID() error {
	id, changed, err := sharedjellyfin.EnsureID(store.value.JellyfinID, sharedjellyfin.NewID)
	if err != nil || !changed {
		return err
	}
	store.value.JellyfinID = id
	return store.save(store.value)
}
