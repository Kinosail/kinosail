package server

import settingsops "github.com/MikeO7/kinosail/packages/settings"

func (store *settingsStore) setDLNA(enabled bool) error {
	if err := store.editable("dlna.enabled"); err != nil {
		return err
	}
	token, err := settingsops.NewDLNAToken(enabled, store.dlnaURL)
	if err != nil {
		return err
	}
	return store.changeInstallationSettings(func(settings *installationSettings) { settings.DLNAToken = token })
}
