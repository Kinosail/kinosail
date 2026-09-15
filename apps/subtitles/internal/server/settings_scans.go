package server

import settingsops "github.com/MikeO7/kinosail/packages/settings"

func (store *settingsStore) setScanFrequency(frequency string) error {
	return settingsops.ChangeScanFrequency(frequency, store.editable, func(valid string) error {
		return store.changeInstallationSettings(func(value *installationSettings) { value.ScanFrequency = valid })
	})
}

func (store *settingsStore) scanFrequency() string {
	return settingsops.Read(&store.mu, func() string { return settingsops.ScanFrequency(store.value.ScanFrequency) })
}
