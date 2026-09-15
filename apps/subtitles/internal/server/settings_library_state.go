package server

func (state libraryFolderState) Editable() error { return state.store.editable("libraries") }

func (state libraryFolderState) Read() []string {
	state.store.mu.RLock()
	defer state.store.mu.RUnlock()
	return append([]string(nil), state.store.value.Libraries...)
}
