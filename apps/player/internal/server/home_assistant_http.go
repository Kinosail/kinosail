package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/homeassistant"
)

func apiHomeAssistantSetting(integration *homeAssistantIntegration) http.HandlerFunc {
	return integration.SettingHandler()
}

func homeAssistantGate(settings *settingsStore, next http.Handler) http.Handler {
	return homeassistant.Gate(settings.homeAssistant, next)
}

func registerHomeAssistant(mux *http.ServeMux, integration *homeAssistantIntegration, auth *authentication) {
	integration.Register(mux, auth.owner, func(writer http.ResponseWriter, request *http.Request, view homeassistant.Approval) error {
		return executeCSRFTemplate(homeAssistantApprovalView, writer, request, view)
	})
}
