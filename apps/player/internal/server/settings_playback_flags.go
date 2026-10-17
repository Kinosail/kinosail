package server

func (store *settingsStore) subtitlesDefault() bool {
	return store.playbackDefaults().SubtitlesDefault()
}

func (store *settingsStore) subtitleMode() string { return store.playbackDefaults().SubtitleMode() }
func (store *settingsStore) autoplay() bool       { return store.playbackDefaults().AutoplayDefault() }
