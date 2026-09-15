package server

import "net/http"

func registerSecuritySettings(mux *http.ServeMux, settings *settingsStore, auth *authentication) {
	mux.Handle("POST /settings/mfa", auth.owner(saveMFARequirement(auth)))
	mux.Handle("POST /settings/session-timeouts", auth.owner(saveSessionTimeouts(settings)))
}
