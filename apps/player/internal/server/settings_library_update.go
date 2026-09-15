package server

func (state libraryFolderState) Update(change func([]string) ([]string, error)) error {
	state.store.mu.Lock()
	defer state.store.mu.Unlock()
	return state.updateLocked(change)
}
