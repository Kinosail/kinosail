package server

import settingsops "github.com/MikeO7/kinosail/packages/settings"

func (store *settingsStore) setPlayback(mode string, autoplay bool, subtitles string, autoSkip []string) error {
	input := settingsops.Playback{PlaybackMode: mode, Autoplay: autoplay, Subtitles: subtitles, AutoSkip: autoSkip}
	return settingsops.ChangePlayback(input, store.editable, func(valid settingsops.Playback) error {
		return store.changeInstallationSettings(func(value *installationSettings) { value.Playback = valid.Merge(value.Playback) })
	})
}
