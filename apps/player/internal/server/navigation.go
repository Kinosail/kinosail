package server

import sharednavigation "github.com/MikeO7/kinosail/packages/navigation"

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

type sidebarGroup struct {
	ID, Name string
	Links    []navigationLink
}

func sidebarGroups(primary, more []navigationLink) []sidebarGroup {
	groups := []sidebarGroup{{ID: "library", Name: "Library"}, {ID: "personal", Name: "Your library"}, {ID: "viewing", Name: "Library views"}}
	for _, links := range [][]navigationLink{primary, more} {
		for _, link := range links {
			index := 0
			switch link.Group {
			case "personal":
				index = 1
			case "viewing":
				index = 2
			}
			groups[index].Links = append(groups[index].Links, link)
		}
	}
	visible := make([]sidebarGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Links) > 0 {
			visible = append(visible, group)
		}
	}
	return visible
}
