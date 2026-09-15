package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

var (
	updateStatusText        = updatecontrol.StatusText
	updateManagerStatusText = updatecontrol.ManagerStatusText
)

func updateHTTP(checker *updateChecker) *updatecontrol.HTTPHandlers {
	handlers, err := updatecontrol.NewHTTPHandlers(checker, updatecontrol.HTTPConfig{ReadJSON: readJSON, JSON: writeJSON, APIError: apiError, WebError: localizedError})
	if err != nil {
		panic(err)
	}
	return handlers
}

func registerUpdateAPI(owner func(string, http.Handler), checker *updateChecker) {
	handlers := updateHTTP(checker)
	owner("GET /api/v1/updates", http.HandlerFunc(handlers.View))
	owner("POST /api/v1/updates", http.HandlerFunc(handlers.Request))
	owner("PUT /api/v1/settings/updates", http.HandlerFunc(handlers.Preference))
	owner("POST /api/v1/updates/check", http.HandlerFunc(handlers.Check))
}

func saveUpdatePreference(checker *updateChecker, redirect string) http.HandlerFunc {
	return updateHTTP(checker).SavePreference(redirect)
}

func checkForUpdate(checker *updateChecker, redirect string) http.HandlerFunc {
	return updateHTTP(checker).CheckAndRequest(redirect)
}
