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

func protectApplicationTransport(auth *authentication, config Config, pattern func(*http.Request) string, handler http.Handler) http.Handler {
	return trustedProxy(config.ProxyToken, allowedHost(config.AuthURL, config.Configuration.Strings("tls.hosts"), observeRequests(auth.audit, pattern, tripwirePublic(auth.audit, publicRequestLimits(handler)))))
}
