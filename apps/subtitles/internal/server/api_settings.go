package server

import (
	"net/http"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
)

func apiSettings(api apiServices) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		value := api.settings.snapshot()
		items, scanned, scanErr := api.index.Status()
		watching, _ := api.index.Monitoring()
		trustedStatus := trustedHTTPSStatus(api.trusted)
		updates := api.updates.View()
		scanMessage := ""
		if scanErr != nil {
			scanMessage = scanErr.Error()
		}
		inactive, absolute := sessionTimeoutHours(value)
		publicInactive, publicAbsolute := publicSessionTimeoutHours(value)
		writeJSON(writer, map[string]any{"name": value.Name, "mediaRoot": api.settings.mediaRoot, "libraries": value.Libraries, "navigation": value.Navigation, "subtitleLanguage": api.settings.subtitleLanguage(), "subtitleLanguages": api.settings.subtitleLanguages(), "subtitleLanguageCatalog": subtitlelanguage.Catalog(), "scanFrequency": api.settings.scanFrequency(), "libraryMonitoring": map[bool]string{true: "watching", false: "polling"}[watching], "requireMfa": value.RequireMFA, "onboardingPending": value.OnboardingPending, "sso": api.auth.sso, "notifications": api.auth.notify.status, "subtitleProvider": api.subtitles.label(), "metadataProvider": api.metadata.label(), "libraryItems": items, "lastScan": scanned, "scanError": scanErr != nil, "scanErrorMessage": scanMessage, "sessions": api.auth.profiles.activeSessions(), "sessionInactiveHours": inactive, "sessionAbsoluteHours": absolute, "publicSessionInactiveHours": publicInactive, "publicSessionAbsoluteHours": publicAbsolute, "publicSessionTimeoutsConfigured": value.PublicSessionAbsoluteHours != 0, "trustedHttps": map[string]any{"configuration": api.settings.trustedHTTPS(), "status": trustedStatus}, "updates": updates}, http.StatusOK)
	}
}
