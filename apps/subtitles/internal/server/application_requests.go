package server

import (
	"net/http"
)

func applicationRequests(auth *authentication, settings *settingsStore, updates *updateChecker, pattern func(*http.Request) string, root http.Handler) http.Handler {
	shell := withApplicationShell(settings, updates, pattern, root)
	return security(localized(auth.protect(shell, pattern)))
}
