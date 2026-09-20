package server

import (
	_ "embed"
	"net/http"

	"github.com/MikeO7/kinosail/packages/webassets"
)

var (
	//go:embed static/htmx.min.js
	htmx []byte
	//go:embed static/player_streaming_start.js
	playerStreamingStartJS []byte
	//go:embed static/player_streaming_end.js
	playerStreamingEndJS []byte
	//go:embed static/supporter.js
	supporterAppJS []byte
	//go:embed static/supporter.css
	supporterCSS []byte
	//go:embed static/subtitle-status.js
	subtitleStatusJS []byte
	//go:embed static/subtitle-inspector.js
	subtitleInspectorJS []byte
	//go:embed static/subtitle-inspector.css
	subtitleInspectorCSS []byte
	//go:embed static/subtitle-dashboard.css
	subtitleDashboardBaseCSS []byte
	//go:embed static/subtitle-workspace.css
	subtitleWorkspaceCSS []byte
	subtitleDashboardCSS = append(append([]byte(nil), subtitleDashboardBaseCSS...), subtitleWorkspaceCSS...)
	//go:embed static/hls.min.js
	hlsJS []byte
	//go:embed static/manifest.webmanifest
	manifest []byte
	//go:embed static/icon.svg
	icon []byte
	//go:embed static/icon-192.png
	icon192 []byte
	//go:embed static/icon-512.png
	icon512 []byte
	//go:embed static/icon-maskable-512.png
	iconMaskable512 []byte
	//go:embed static/apple-touch-icon.png
	appleTouchIcon []byte
	//go:embed static/cinema-backdrop.jpg
	cinemaBackdrop []byte
	//go:embed static/service-worker.js
	serviceWorkerApp []byte
	//go:embed static/offline.html
	offlineHTML []byte
)

var offlineView = newLocalizedTemplate("offline", string(offlineHTML))

func serveOffline(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	_ = offlineView.Execute(writer, request, nil)
}

func serveScript(content []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		cacheStatic(writer, request)
		_, _ = writer.Write(content)
	}
}

var (
	passkeysJS          = webassets.Passkeys
	downloadsCoreJS     = webassets.Downloads
	downloadsTransferJS = webassets.DownloadsTransfer
	downloadsUIJS       = webassets.DownloadsUI
	playerCoreJS        = webassets.PlayerCore
	playerControlsJS    = webassets.PlayerControls
	playerDevicesJS     = webassets.PlayerDevices
	playerProgressJS    = webassets.PlayerProgress
	pwaCoreJS           = webassets.PWA
	pwaNavigationJS     = webassets.PWANavigation
	pwaSettingsJS       = webassets.PWASettings
	shortcutsJS         = webassets.Shortcuts
	themeJS             = joinScripts(webassets.Theme, webassets.ArtworkPalette, webassets.WatchProgress)
	supporterJS         = joinScripts(webassets.Supporter, supporterAppJS)
	playerJS            = joinScripts(playerCoreJS, playerStreamingStartJS, playerStreamingEndJS, playerControlsJS, playerDevicesJS, playerProgressJS)
	downloadsJS         = joinScripts(webassets.OfflineIdentity, webassets.OfflineRuntime, downloadsCoreJS, webassets.DownloadsStorage, downloadsTransferJS, webassets.DownloadsProgress, downloadsUIJS)
	serviceWorker       = joinScripts(webassets.OfflineRuntime, webassets.OfflineMedia, serviceWorkerApp)
	pwaJS               = joinScripts(webassets.OfflineIdentity, pwaCoreJS, pwaNavigationJS, pwaSettingsJS)
	mainBundle          = joinScripts(pwaJS, shortcutsJS)
)

func joinScripts(scripts ...[]byte) []byte {
	var result []byte
	for _, script := range scripts {
		result = append(result, script...)
	}
	return result
}

func serveServiceWorker(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Service-Worker-Allowed", "/")
	_, _ = writer.Write(serviceWorker)
}

func serveStyle(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/css; charset=utf-8")
	cacheStatic(writer, request)
	_, _ = writer.Write(appCSS)
	_, _ = writer.Write(webassets.LastLightCSS)
	_, _ = writer.Write(supporterCSS)
	_, _ = writer.Write(subtitleDashboardCSS)
}

func serveSupporterStyle(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/css; charset=utf-8")
	cacheStatic(writer, request)
	_, _ = writer.Write(supporterCSS)
}

func serveAsset(content []byte, contentType string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		cacheStatic(writer, request)
		_, _ = writer.Write(content)
	}
}

func cacheStatic(writer http.ResponseWriter, request *http.Request) {
	value := "public, max-age=86400"
	if request.URL.Query().Get("v") != "" {
		value = "public, max-age=31536000, immutable"
	}
	writer.Header().Set("Cache-Control", value)
}
