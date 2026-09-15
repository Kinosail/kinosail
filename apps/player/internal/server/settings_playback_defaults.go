package server

import settingsops "github.com/MikeO7/kinosail/packages/settings"

func (store *settingsStore) playbackDefaults() settingsops.Playback {
	return settingsops.Read(&store.mu, func() settingsops.Playback { return store.value.Playback })
}

func (store *settingsStore) playbackMode() string {
	return settingsops.Read(&store.mu, func() string { return normalizePlayback(store.value.PlaybackMode) })
}
func normalizePlayback(mode string) string { return settingsops.PlaybackMode(mode) }
