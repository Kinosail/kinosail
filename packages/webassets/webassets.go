// Package webassets owns browser behavior shared by Kinosail applications.
package webassets

import _ "embed"

var (
	//go:embed static/fonts/manrope.woff2
	Manrope []byte

	//go:embed static/watch-progress.js
	WatchProgress []byte

	//go:embed static/last-light.css
	LastLightCSS []byte
	//go:embed static/artwork-palette.js
	ArtworkPalette []byte
	//go:embed static/downloads.js
	Downloads []byte
	//go:embed static/downloads-storage.js
	DownloadsStorage []byte

	//go:embed static/downloads-transfer.js
	DownloadsTransfer []byte
	//go:embed static/downloads-progress.js
	DownloadsProgress []byte
	//go:embed static/downloads-ui.js
	DownloadsUI []byte
	//go:embed static/offline-media.js
	OfflineMedia []byte

	//go:embed static/offline-runtime.js
	OfflineRuntime []byte
	//go:embed static/passkeys.js
	Passkeys []byte
	//go:embed static/public-login.js
	PublicLogin []byte
	//go:embed static/player-controls.js
	PlayerControls []byte
	//go:embed static/player-core.js
	PlayerCore []byte
	//go:embed static/player-devices.js
	PlayerDevices []byte
	//go:embed static/player-tv.js
	PlayerTV []byte
	//go:embed static/player-progress.js
	PlayerProgress []byte
	//go:embed static/pwa.js
	PWA []byte
	//go:embed static/mobile-tabs.js
	MobileTabs []byte
	//go:embed static/pwa-navigation.js
	PWANavigation []byte
	//go:embed static/pwa-settings.js
	PWASettings []byte
	//go:embed static/shortcuts.js
	Shortcuts []byte
	//go:embed static/supporter.js
	Supporter []byte
	//go:embed static/appearance.js
	Appearance []byte
	//go:embed static/theme.js
	themeControls []byte
	Theme         = append(append([]byte(nil), Appearance...), themeControls...)
)
