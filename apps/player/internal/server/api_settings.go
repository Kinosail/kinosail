package server

import "net/http"

func apiSettings(api apiServices) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		value := api.settings.snapshot()
		items, scanned, scanErr := api.index.Status()
		watching, _ := api.index.Monitoring()
		cache, cacheErr := api.hls.cacheOps.Stats()
		trustedStatus := trustedHTTPSStatus(api.trusted)
		updates := api.updates.View()
		scanMessage := ""
		if scanErr != nil {
			scanMessage = scanErr.Error()
		}
		inactive, absolute := sessionTimeoutHours(value)
		transcoding := api.settings.transcoding()
		writeJSON(writer, map[string]any{"name": value.Name, "mediaRoot": api.settings.mediaRoot, "libraries": value.Libraries, "navigation": value.Navigation, "playbackMode": normalizePlayback(value.PlaybackMode), "autoplay": api.settings.autoplay(), "autoSkip": api.settings.autoSkip(), "subtitles": value.Subtitles, "subtitleLanguage": api.settings.subtitleLanguage(), "transcoder": transcoding.Name, "codec": normalizeCodecSetting(value.Codec), "effectiveCodec": transcoding.Codec, "accelerator": normalizeAccelerator(value.Accelerator), "effectiveAccelerator": transcoding.Accelerator, "hardware": api.settings.hardware, "transcoderTest": api.settings.currentTranscoderCheck(), "toneMap": value.ToneMap, "scanFrequency": api.settings.scanFrequency(), "libraryMonitoring": map[bool]string{true: "watching", false: "polling"}[watching], "dlna": value.DLNAToken != "", "dlnaAvailable": api.settings.dlnaURL != "", "jellyfinCompatibility": api.settings.jellyfinCompatibility(), "jellyfinURL": trustedConnectionURL(api.authURL, api.settings.trustedHTTPS().Hostname), "homeAssistant": value.HomeAssistant, "requireMfa": value.RequireMFA, "onboardingPending": value.OnboardingPending, "sso": api.auth.sso, "notifications": api.auth.notify.status, "metadataProvider": api.metadata.label(), "libraryItems": items, "lastScan": scanned, "scanError": scanErr != nil, "scanErrorMessage": scanMessage, "sessions": api.auth.profiles.activeSessions(), "sessionInactiveHours": inactive, "sessionAbsoluteHours": absolute, "transcodeCacheBytes": cache, "cacheError": cacheErr != nil, "trustedHttps": map[string]any{"configuration": api.settings.trustedHTTPS(), "status": trustedStatus}, "updates": updates}, http.StatusOK)
	}
}
