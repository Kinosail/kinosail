package server

import (
	"embed"
	"net/http"
	"path"

	"github.com/MikeO7/kinosail/packages/webassets"
)

var (
	//go:embed static/htmx.min.js
	htmx []byte
	//go:embed static/player-subtitles.js
	playerSubtitlesJS []byte
	//go:embed static/player.js
	playerCoreJS []byte
	//go:embed static/player-streaming-adaptive.js
	playerStreamingAdaptiveJS []byte
	//go:embed static/player-streaming-recovery.js
	playerStreamingRecoveryJS []byte
	//go:embed static/player-streaming-offline.js
	playerStreamingOfflineJS []byte
	//go:embed static/supporter.js
	supporterAppJS []byte
	//go:embed static/supporter-plans.js
	supporterPlansJS []byte
	//go:embed static/supporter.css
	supporterCSS []byte
	//go:embed static/home.css
	homeCSS []byte
	//go:embed static/connect.js
	connectJS []byte
	//go:embed static/third_party/jsqr/jsQR.js
	qrDecoderJS []byte
	//go:embed static/quick-connect-scan.js
	quickConnectScanJS []byte
	//go:embed static/quick-connect.js
	quickConnectJS []byte
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
	//go:embed static/supporter/badges/*
	supporterBadges embed.FS
)

var (
	passkeysJS          = webassets.Passkeys
	downloadsCoreJS     = webassets.Downloads
	downloadsTransferJS = webassets.DownloadsTransfer
	downloadsUIJS       = webassets.DownloadsUI
	playerSharedJS      = webassets.PlayerCore
	playerControlsJS    = webassets.PlayerControls
	playerDevicesJS     = webassets.PlayerDevices
	playerProgressJS    = webassets.PlayerProgress
	pwaCoreJS           = webassets.PWA
	pwaNavigationJS     = webassets.PWANavigation
	pwaSettingsJS       = webassets.PWASettings
	shortcutsJS         = webassets.Shortcuts
	themeJS             = joinScripts(webassets.Theme, webassets.ArtworkPalette, webassets.WatchProgress)
	supporterJS         = joinScripts(webassets.Supporter, supporterAppJS, supporterPlansJS)
	playerJS            = joinScripts(playerSharedJS, playerSubtitlesJS, playerCoreJS, playerStreamingAdaptiveJS, playerStreamingRecoveryJS, playerStreamingOfflineJS, playerControlsJS, playerDevicesJS, webassets.PlayerTV, playerProgressJS)
	downloadsJS         = joinScripts(webassets.OfflineRuntime, downloadsCoreJS, webassets.DownloadsStorage, downloadsTransferJS, webassets.DownloadsProgress, downloadsUIJS)
	serviceWorker       = joinScripts(webassets.OfflineRuntime, webassets.OfflineMedia, serviceWorkerApp)
	pwaJS               = joinScripts(pwaCoreJS, pwaNavigationJS, pwaSettingsJS, webassets.MobileTabs)
)

func joinScripts(parts ...[]byte) []byte {
	var script []byte
	for _, part := range parts {
		script = append(script, part...)
	}
	return script
}

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

var mainBundle = append(append([]byte(nil), pwaJS...), shortcutsJS...)

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
	_, _ = writer.Write(homeCSS)
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
		_, _ = writer.Write(content) //nolint:gosec // G705: every caller supplies embedded asset bytes, never reflected request content.
	}
}

func serveSupporterBadge(writer http.ResponseWriter, request *http.Request) {
	name := path.Base(request.URL.Path)
	if path.Ext(name) != ".svg" {
		http.NotFound(writer, request)
		return
	}
	content, err := supporterBadges.ReadFile("static/supporter/badges/" + name)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	serveAsset(content, "image/svg+xml")(writer, request)
}

func cacheStatic(writer http.ResponseWriter, request *http.Request) {
	value := "public, max-age=86400"
	if request.URL.Query().Get("v") != "" {
		value = "public, max-age=31536000, immutable"
	}
	writer.Header().Set("Cache-Control", value)
}
