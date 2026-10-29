package server

func (store *settingsStore) setSessionTimeouts(inactive, absolute float64) error {
	return store.changeSessionTimeouts(inactive, absolute, false)
}
