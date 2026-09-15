package server

func (store *settingsStore) autoSkip() []string { return store.playbackDefaults().AutoSkipDefault() }
