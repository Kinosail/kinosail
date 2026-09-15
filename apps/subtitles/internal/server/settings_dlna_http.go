package server

import (
	"net/http"

	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

func dlnaLabel(settings *settingsStore) string {
	return settingsops.DLNALabel(settings.dlnaURL, settings.dlnaToken())
}

func saveDLNA(settings *settingsStore) http.HandlerFunc {
	return settingsops.SaveDLNA(settings.setDLNA, localizedError)
}
