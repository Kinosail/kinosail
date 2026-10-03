package playerweb

import "strings"

// StablePlayerLayout renders the native controls' final structure before scripts
// load. Custom controls retain their established inline layout.
func StablePlayerLayout(source string) string {
	anchor := `<p class="playback-device-status"`
	if !strings.Contains(source, anchor) {
		anchor = `<div class="primary-player-actions">`
	}
	toolbarStart := strings.Index(source, `<div class="player-stage-toolbar">`)
	controlsStart := strings.Index(source, `<div class="player-controls`)
	settingsStart := strings.Index(source, `<div class="player-settings"`)
	bufferStart := strings.Index(source, `<div class="player-buffer"`)
	if !strings.Contains(source, anchor) || toolbarStart < 0 || controlsStart <= toolbarStart || settingsStart <= controlsStart || bufferStart <= settingsStart {
		return source
	}
	toolbar, settings := source[toolbarStart:controlsStart], source[settingsStart:bufferStart]
	settingsButtonStart := strings.Index(source, `<button type="button" aria-label="Settings"`)
	if settingsButtonStart < 0 {
		return source
	}
	closingButton := strings.Index(source[settingsButtonStart:], `</button>`)
	if closingButton < 0 {
		return source
	}
	settingsButtonEnd := settingsButtonStart + closingButton + len(`</button>`)
	settingsButton := source[settingsButtonStart:settingsButtonEnd]
	toolbar = strings.TrimSuffix(toolbar, `</div>`) + settingsButton + `</div>`
	source = strings.Replace(source, settingsButton, `{{if not .NativeControls}}`+settingsButton+`{{end}}`, 1)
	source = strings.Replace(source, settings, `{{if not .NativeControls}}`+settings+`{{end}}`, 1)
	// The unmodified toolbar is still present once in the original stage.
	originalToolbar := source[toolbarStart:controlsStart]
	source = strings.Replace(source, originalToolbar, `{{if not .NativeControls}}`+originalToolbar+`{{end}}`, 1)
	options := `{{if and (eq .Kind "video") .NativeControls}}<div class="player-native-options">` + toolbar + settings + `</div>{{end}}`
	source = strings.Replace(source, anchor, options+anchor, 1)
	source = strings.Replace(source, `{{if not .SubtitlePickerLimited}}controls data-native-controls {{end}}`, `{{if not .SubtitlePickerLimited}}{{if not .AppleNativeControls}}controls {{end}}data-native-controls {{end}}`, 1)
	source = strings.Replace(source, `id="player-media" controls playsinline`, `id="player-media" {{if not .AppleNativeControls}}controls {{end}}playsinline`, 1)
	source = strings.Replace(source, `{{if .Resume}}data-autoplay{{else}}autoplay{{end}}`, `{{if not .AppleNativeControls}}{{if .Resume}}data-autoplay{{else}}autoplay{{end}}{{end}}`, 1)
	source = strings.Replace(source, `<div class="player-controls`, `<div class="player-controls{{if .AppleNativeControls}} player-native-controls{{end}}`, 1)
	return source
}
