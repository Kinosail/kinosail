package server

func (state libraryFolderState) updateLocked(change func([]string) ([]string, error)) error {
	settings := state.store.value
	libraries, err := change(append([]string(nil), settings.Libraries...))
	if err != nil {
		return err
	}
	settings.Libraries = libraries
	if err := state.store.save(settings); err != nil {
		return err
	}
	state.store.value = settings
	return nil
}
