package server

import sharednavigation "github.com/MikeO7/kinosail/packages/navigation"

type (
	navigationLink       = sharednavigation.Link
	navigationPreference = sharednavigation.Preference
)

func (store *settingsStore) navigation() []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return append([]string(nil), store.value.Navigation...)
}

func (store *settingsStore) setNavigation(items []string) error {
	if err := sharednavigation.Validate(items); err != nil {
		return err
	}
	return store.changeInstallationSettings(func(settings *installationSettings) {
		settings.Navigation = append([]string(nil), items...)
	})
}
