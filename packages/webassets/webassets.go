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
	//go:embed static/offline-profile.js
	offlineProfile []byte
	//go:embed static/shell-assets.js
	ShellAssets []byte
	//go:embed static/offline-shell.js
	offlineShellLifecycle []byte
	offlineShell          = append(append([]byte(nil), ShellAssets...), offlineShellLifecycle...)
	OfflineShell          = append(append([]byte(nil), offlineProfile...), offlineShell...)

	//go:embed static/offline-runtime.js
	OfflineRuntime []byte
	//go:embed static/offline-identity.js
	OfflineIdentity []byte
	//go:embed static/passkeys.js
	Passkeys []byte
	//go:embed static/public-login.js
	PublicLogin []byte
	//go:embed static/player-controls.js
	playerControlsCore []byte
	//go:embed static/player-preview.js
	playerSeekPreview []byte
	playerControls    = append(append([]byte(nil), playerSeekPreview...), playerControlsCore...)
	//go:embed static/player-status.js
	playerStatus []byte
	//go:embed static/player-presentation.js
	playerPresentation []byte

	playerStatusControls = append(append([]byte(nil), playerControls...), playerStatus...)
	PlayerControls       = append(append([]byte(nil), playerStatusControls...), playerPresentation...)
	//go:embed static/player-core.js
	PlayerCore []byte
	//go:embed static/player-devices.js
	PlayerDevices []byte
	//go:embed static/player-tv.js
	PlayerTV []byte
	//go:embed static/player-progress.js
	playerProgress []byte
	//go:embed static/player-progress-navigation.js
	playerProgressNavigation []byte
	//go:embed static/player-audio-queue.js
	playerAudioQueue []byte

	playerProgressQueue = append(append([]byte(nil), playerProgress...), playerAudioQueue...)
	PlayerProgress      = append(append([]byte(nil), playerProgressQueue...), playerProgressNavigation...)
	//go:embed static/pwa.js
	pwaCore []byte
	//go:embed static/pwa-library.js
	pwaLibrary []byte
	PWA        = append(append([]byte(nil), pwaLibrary...), pwaCore...)
	//go:embed static/dock-focus.js
	DockFocus []byte
	//go:embed static/mobile-tabs.js
	mobileTabs []byte
	MobileTabs = append(append([]byte(nil), DockFocus...), mobileTabs...)
	//go:embed static/pwa-navigation.js
	pwaNavigation []byte
	//go:embed static/pwa-browse-return.js
	pwaBrowseReturn []byte
	PWANavigation   = append(append([]byte(nil), pwaNavigation...), pwaBrowseReturn...)
	//go:embed static/pwa-settings.js
	PWASettings []byte
	//go:embed static/library-loading.js
	LibraryLoading []byte
	//go:embed static/shortcuts.js
	Shortcuts []byte
	//go:embed static/supporter.js
	Supporter []byte
	//go:embed static/appearance.js
	Appearance []byte
	//go:embed static/theme.js
	themeControls []byte
	//go:embed static/player-layout-initial.js
	playerLayoutInitial []byte
	//go:embed static/fragment-layout-initial.js
	fragmentLayoutInitial []byte
	themeAppearance       = append(append([]byte(nil), Appearance...), themeControls...)
	themePlayerLayout     = append(append([]byte(nil), themeAppearance...), playerLayoutInitial...)
	Theme                 = append(append([]byte(nil), themePlayerLayout...), fragmentLayoutInitial...)
)
