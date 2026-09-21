package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/homeassistant"
)

func applicationRequests(auth *authentication, settings *settingsStore, updates *updateChecker, pattern func(*http.Request) string, root http.Handler) http.Handler {
	shell := withApplicationShell(settings, updates, pattern, root)
	authenticated := homeassistant.Gate(settings.homeAssistant, auth.protect(shell, pattern))
	return security(localized(jellyfinCompatibility(settings, authenticated)))
}
