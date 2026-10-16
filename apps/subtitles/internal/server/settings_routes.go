package server

import (
	"net/http"

	settingsops "github.com/MikeO7/kinosail/packages/settings"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func registerSettings(mux *http.ServeMux, settings *settingsStore, updates *updateChecker, index *libraryIndex, auth *authentication, trusted *trustedhttps.Manager, subtitles *subtitleProvider, metadata *metadataStore, backups *backupManager, maintenance *maintenanceManager, events *liveEventHub) {
	registerOperations(mux, auth, settings, index, metadata, maintenance, events)
	mux.Handle("GET /onboarding/finish", auth.owner(finishOnboarding(settings)))
	registerSubtitleOnboardingActions(mux, auth, settings, index)
	registerSubtitleOnboarding(mux, auth, settings, subtitles)
	mux.Handle("POST /api/v1/subtitles/cleanup/preview", auth.owner(apiPreviewSubtitleCleanup(index, settings)))
	mux.Handle("POST /api/v1/subtitles/cleanup", auth.owner(apiApplySubtitleCleanup(index, settings)))
	mux.Handle("GET /settings", auth.owner(showSubtitleSettings(settings, subtitles, trusted)))
	mux.Handle("GET /settings/subtitles/cleanup", auth.owner(previewSubtitleCleanup(index, settings)))
	mux.Handle("POST /settings/subtitles/cleanup", auth.owner(hideSubtitleCleanup(index, settings)))
	mux.Handle("POST /settings/updates", auth.owner(saveUpdatePreference(updates, "/settings#updates")))
	mux.Handle("POST /settings/updates/check", auth.owner(checkForUpdate(updates, "/settings#updates")))
	mux.Handle("POST /settings/trusted-https", auth.owner(saveTrustedHTTPS(settings, "/settings#trusted-https")))
	mux.Handle("POST /settings/trusted-https/disable", auth.owner(disableTrustedHTTPS(settings)))
	mux.Handle("GET /settings/system", auth.owner(showSystem(settings, index, auth.profiles, auth.audit)))
	mux.Handle("GET /settings/configuration", auth.owner(showConfiguration(settings)))
	mux.Handle("POST /settings/configuration", auth.owner(saveConfiguration(settings, false)))
	mux.Handle("POST /settings/configuration/reset", auth.owner(saveConfiguration(settings, true)))
	mux.Handle("GET /settings/backups", auth.owner(settingsops.ShowBackups(backups.Status, backupsView.Execute, localizedError)))
	mux.Handle("POST /settings/backups/verify", auth.owner(settingsops.VerifyBackup(backups.Verify, localizedError)))
	mux.Handle("POST /settings/libraries", auth.owner(changeLibrary(settings, index, settings.add, "/settings")))
	mux.Handle("POST /settings/libraries/remove", auth.owner(changeLibrary(settings, index, settings.remove, "/settings")))
	mux.Handle("POST /settings/server", auth.owner(saveServerName(settings)))
	mux.Handle("POST /settings/onboarding", auth.owner(restartOnboarding(settings)))
	registerSecuritySettings(mux, settings, auth)
	mux.Handle("POST /settings/subtitles", auth.owner(saveSubtitleLanguage(settings, "/settings#language")))
	registerSubSourceConfiguration(mux, auth, settings)
	mux.Handle("POST /settings/scans", auth.owner(saveScanFrequency(settings, index, "/settings")))
	mux.Handle("GET /settings/backup", auth.freshOwner(downloadBackup(settings)))
	mux.Handle("POST /settings/encrypted-backup", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := backups.WriteNow(); err != nil {
			localizedError(writer, request, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Redirect(writer, request, "/settings/backups", http.StatusSeeOther)
	})))
	mux.Handle("POST /settings/sessions/revoke", auth.owner(revokeSessions(auth.profiles)))
	mux.Handle("POST /settings/sessions/device", auth.owner(revokeDevice(auth.profiles)))
	mux.Handle("POST /settings/api-keys", auth.owner(createAPIKey(auth.profiles)))
	mux.Handle("POST /settings/api-keys/revoke", auth.owner(revokeAPIKey(auth.profiles)))
}
