package quickconnect

// Page contains product branding for the shared Quick Connect page.
type Page struct {
	Product, ThemeScript, Stylesheet, Icon string
	AutoSubmit                             bool
	Code, Device                           string
}

// HTML is Player's canonical Quick Connect page.
const HTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Quick Connect · {{.Product}}</title><script src="{{.ThemeScript}}"></script><link rel="stylesheet" href="{{.Stylesheet}}">{{if .AutoSubmit}}<script defer src="/static/quick-connect-scan.js?v=1"></script><script defer src="/static/quick-connect.js?v=1"></script>{{end}}</head><body class="auth"><main><form method="post" action="/quick-connect"><img class="brand-mark" src="{{.Icon}}" alt=""><span class="eyebrow">Quick Connect</span><h1>Connect a device</h1>{{if .Code}}<p>Approve <strong>{{.Device}}</strong> using this profile’s access and content settings.</p><p>Only approve if this code matches your TV: <strong>{{.Code}}</strong>.</p><input type="hidden" name="code" value="{{.Code}}">{{else}}<p>Enter the six-digit code shown by your TV or app. It will use this profile’s access and content settings.</p>{{if .AutoSubmit}}<button type="button" class="quiet" data-qr-start hidden>Scan QR code</button><div data-qr-camera hidden><video data-qr-video autoplay muted playsinline aria-label="QR scanner camera preview"></video><button type="button" class="quiet" data-qr-stop>Stop scanning</button></div><p role="status" data-qr-status hidden></p><fieldset class="quick-connect-code-field"><legend>Code</legend><div class="quick-connect-code" role="group" aria-label="Six-digit code"><input autofocus required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="one-time-code" aria-label="Digit 1 of 6"><input required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="off" aria-label="Digit 2 of 6"><input required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="off" aria-label="Digit 3 of 6"><input required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="off" aria-label="Digit 4 of 6"><input required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="off" aria-label="Digit 5 of 6"><input required class="quick-connect-digit" data-quick-connect-digit type="text" inputmode="numeric" maxlength="1" pattern="[0-9]" autocomplete="off" aria-label="Digit 6 of 6"></div><input type="hidden" name="code" data-quick-connect-value></fieldset>{{else}}<label>Code<input autofocus required name="code" minlength="6" maxlength="6" inputmode="numeric" pattern="[0-9]{6}" autocomplete="one-time-code"></label>{{end}}{{end}}<button>{{if .Code}}Approve device{{else}}Authorize device{{end}}</button><a href="/">Cancel</a></form></main></body></html>` //nolint:lll // Compact source preserves Player's response body.

// PlayerPage returns Player's canonical Quick Connect product page.
func PlayerPage() Page {
	return Page{Product: "Kinosail Player", ThemeScript: "/static/theme.js?v=cinema-1", Stylesheet: "/static/app.css?v=qr-scan-1", Icon: "/static/icon.svg?v=6", AutoSubmit: true}
}

// SubtitlesPage returns Subtitles branding for Player's shared flow.
func SubtitlesPage() Page {
	return Page{Product: "Kinosail Subtitles", ThemeScript: "/static/theme.js?v=cinema-1", Stylesheet: "/static/app.css?v=cinema-1", Icon: "/static/icon.svg?v=9"}
}
