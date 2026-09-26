package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func registerSettings(mux *http.ServeMux, settings *settingsStore, updates *updateChecker, index *libraryIndex, progress *progressStore, auth *authentication, internet *remoteaccess.Manager, trusted *trustedhttps.Manager, quick *quickConnectBroker, shares *mediaShareStore, hls *hlsManager, subtitles *subtitleProvider, metadata *metadataStore, markers *markerAnalyzer, backups *backupManager, maintenance *maintenanceManager, imports *viewingImportManager, homeAssistant *homeAssistantIntegration, events *liveEventHub, rooms *watchRoomAdapter, authURL string, subtitleApp bool) { //nolint:funlen // Keep settings routes together for review.
	registerOperations(mux, auth, settings, index, hls, metadata, maintenance, imports, events, rooms)
	mux.Handle("GET /onboarding/finish", auth.owner(finishOnboarding(settings)))
	registerSubtitleOnboardingActions(mux, auth, settings, index)
	if subtitleApp {
		registerSubtitleOnboarding(mux, auth, settings, subtitles)
	} else {
		mux.Handle("GET /onboarding", auth.owner(http.HandlerFunc(showOnboardingStart)))
		mux.Handle("GET /onboarding/connection", auth.owner(showOnboardingConnection(settings, updates, authURL)))
		mux.Handle("GET /onboarding/household", auth.owner(showOnboardingHousehold(auth.profiles)))
		mux.Handle("POST /onboarding/household", auth.owner(addOnboardingViewer(auth.profiles)))
		mux.Handle("POST /onboarding/trusted-https", auth.owner(saveTrustedHTTPS(settings, "/onboarding/connection")))
		mux.Handle("POST /onboarding/jellyfin", auth.owner(saveJellyfinCompatibility(settings, "/onboarding/connection#jellyfin")))
		mux.Handle("POST /onboarding/home-assistant", auth.owner(saveHomeAssistant(homeAssistant, "/onboarding/connection")))
		mux.Handle("POST /onboarding/updates", auth.owner(saveUpdatePreference(updates, "/onboarding/connection#updates")))
		mux.Handle("POST /onboarding/updates/check", auth.owner(checkForUpdate(updates, "/onboarding/connection#updates")))
	}
	settingsPage := showSettings(settings, updates, auth.profiles, internet, trusted, auth.audit, index, hls, subtitles, metadata, markers, maintenance, imports, auth.notify.status, authURL)
	if subtitleApp {
		settingsPage = showSubtitleSettings(settings, subtitles, trusted)
	}
	mux.Handle("GET /settings", auth.owner(settingsPage))
	mux.Handle("GET /settings/subtitles/cleanup", auth.owner(previewSubtitleCleanup(index, settings)))
	mux.Handle("POST /settings/subtitles/cleanup", auth.owner(deleteSubtitleCleanup(index, settings)))
	mux.Handle("POST /settings/updates", auth.owner(saveUpdatePreference(updates, "/settings#updates")))
	mux.Handle("POST /settings/updates/check", auth.owner(checkForUpdate(updates, "/settings#updates")))
	mux.Handle("POST /settings/trusted-https", auth.owner(saveTrustedHTTPS(settings, "/settings#trusted-https")))
	mux.Handle("POST /settings/trusted-https/disable", auth.owner(disableTrustedHTTPS(settings)))
	mux.Handle("GET /settings/remote-readiness", auth.owner(showRemoteReadiness(internet, auth.profiles)))
	mux.Handle("GET /settings/system", auth.owner(showSystem(settings, index, progress, auth.profiles, hls, auth.audit)))
	mux.Handle("GET /settings/configuration", auth.owner(showConfiguration(settings)))
	mux.Handle("POST /settings/configuration", auth.owner(saveConfiguration(settings, false)))
	mux.Handle("POST /settings/configuration/reset", auth.owner(saveConfiguration(settings, true)))
	mux.Handle("GET /settings/backups", auth.owner(settingsops.ShowBackups(backups.Status, backupsView.Execute, localizedError)))
	mux.Handle("POST /settings/backups/verify", auth.owner(settingsops.VerifyBackup(backups.Verify, localizedError)))
	mux.Handle("POST /settings/libraries", auth.owner(changeLibrary(settings, index, settings.add, "/settings")))
	mux.Handle("POST /settings/libraries/remove", auth.owner(changeLibrary(settings, index, settings.remove, "/settings")))
	mux.Handle("POST /settings/server", auth.owner(saveServerName(settings)))
	mux.Handle("POST /settings/navigation", auth.owner(saveNavigation(settings)))
	mux.Handle("POST /settings/onboarding", auth.owner(restartOnboarding(settings)))
	registerSecuritySettings(mux, settings, auth)
	mux.Handle("POST /settings/playback", auth.owner(savePlayback(settings)))
	mux.Handle("POST /settings/transcoder", auth.owner(saveTranscoder(settings)))
	mux.Handle("POST /settings/transcoder/test", auth.owner(transcodepolicy.CheckHandler(settings.transcoderState().CheckCoordinator(settings.ffmpeg, settings.hardware).Run)))
	mux.Handle("POST /settings/subtitles", auth.owner(saveSubtitleLanguage(settings, "/settings#language")))
	registerSubSourceConfiguration(mux, auth, settings)
	mux.Handle("POST /settings/scans", auth.owner(saveScanFrequency(settings, index, "/settings")))
	mux.Handle("POST /settings/dlna", auth.owner(saveDLNA(settings)))
	mux.Handle("POST /settings/jellyfin", auth.owner(saveJellyfinCompatibility(settings, "/settings#jellyfin")))
	mux.Handle("POST /settings/home-assistant", auth.owner(saveHomeAssistant(homeAssistant, "/settings#access")))
	mux.Handle("POST /settings/home-assistant/pair", auth.owner(homeAssistant.pairingPage()))
	mux.Handle("POST /settings/profiles", auth.owner(addViewer(auth.profiles)))
	mux.Handle("POST /settings/profiles/remove", auth.owner(removeViewer(auth.profiles)))
	mux.Handle("POST /settings/profiles/password", auth.owner(resetViewerPassword(auth.profiles)))
	mux.Handle("POST /settings/profiles/permissions", auth.owner(setViewerPermissions(auth.profiles)))
	mux.Handle("POST /settings/remote/kill", auth.owner(remoteaccess.KillHTTP(internet, func() error { return revokePublicAuthorization(auth.profiles, auth.passkeys, quick, shares) }, localizedError))) //nolint:contextcheck // Once the kill switch begins, revocation must finish despite request cancellation.
	mux.Handle("POST /settings/remote/enable", auth.owner(remoteaccess.ResetKillHTTP(internet, localizedError)))
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
	mux.Handle("POST /settings/cache/clear", auth.owner(clearCache(hls)))
}
