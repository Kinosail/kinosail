package server

import (
	"bytes"
	"embed"
	"net/http"
	"regexp"

	"github.com/MikeO7/kinosail/packages/webassets"
)

var (
	//go:embed static/htmx.min.js
	htmx []byte
	//go:embed static/supporter.js
	supporterAppJS []byte
	//go:embed static/supporter.css
	supporterCSS []byte
	//go:embed static/supporter/badges/*.svg
	supporterBadges embed.FS
	//go:embed static/subtitle-status.js
	subtitleStatusJS []byte
	//go:embed static/subtitle-inspector.js
	subtitleInspectorAppJS []byte
	//go:embed static/subtitle-save-operation.js
	subtitleSaveOperationJS []byte
	//go:embed static/subtitle-source-cues.js
	subtitleSourceCuesJS []byte
	subtitleInspectorJS  = joinScripts(subtitleSourceCuesJS, subtitleSaveOperationJS, subtitleInspectorAppJS)
	//go:embed static/subtitle-inspector.css
	subtitleInspectorCSS []byte
	//go:embed static/subtitle-dashboard.css
	subtitleDashboardBaseCSS []byte
	//go:embed static/subtitle-workspace.css
	subtitleWorkspaceCSS []byte
	//go:embed static/subtitle-dock-initial.js
	subtitleDockInitialJS []byte
	subtitleDashboardCSS  = append(append([]byte(nil), subtitleDashboardBaseCSS...), subtitleWorkspaceCSS...)
	//go:embed static/manifest.webmanifest
	manifest []byte
	//go:embed static/icon.svg
	icon []byte
	//go:embed static/tmdb-logo.svg
	tmdbLogo []byte
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
	serviceWorker []byte
	//go:embed static/offline.html
	offlineHTML []byte
)

var offlineView = newLazyLocalizedTemplate("offline", func() string { return string(offlineHTML) })

func serveOffline(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	_ = offlineView.Execute(writer, request, nil)
}

func serveScript(content []byte) http.HandlerFunc {
	return webassets.Static(content, "text/javascript; charset=utf-8")
}

var (
	passkeysJS       = webassets.Passkeys
	pwaCoreJS        = webassets.PWA
	pwaNavigationJS  = webassets.PWANavigation
	pwaSettingsJS    = webassets.PWASettings
	shortcutsJS      = webassets.Shortcuts
	themeJS          = joinScripts(subtitleDockInitialJS, webassets.Theme)
	supporterJS      = joinScripts(webassets.Supporter, supporterAppJS)
	pwaJS            = joinScripts(pwaCoreJS, pwaNavigationJS, pwaSettingsJS)
	mainBundle       = joinScripts(pwaJS, shortcutsJS)
	applicationStyle = bytes.Join([][]byte{appCSS, webassets.LastLightCSS, supporterCSS, subtitleDashboardCSS}, nil)
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

var serveStyle = webassets.Static(applicationStyle, "text/css; charset=utf-8")

var serveSupporterStyle = webassets.Static(supporterCSS, "text/css; charset=utf-8")

func serveAsset(content []byte, contentType string) http.HandlerFunc {
	return webassets.Static(content, contentType)
}

var supporterBadgeFile = regexp.MustCompile(`^(living-standard|patron-order)-(?:[1-9]|10)\.svg$`)

func serveSupporterBadge(writer http.ResponseWriter, request *http.Request) {
	name := request.PathValue("file")
	if !supporterBadgeFile.MatchString(name) {
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
