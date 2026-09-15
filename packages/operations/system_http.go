package operations

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

// SystemBrand contains the only application-specific values in the System view.
type SystemBrand struct {
	Product, IconVersion, ThemeVersion, StylesheetVersion string
}

const systemViewSource = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#090a08"><link rel="icon" href="/static/icon.svg?v=__ICON__"><title>System · Kinosail __PRODUCT__</title><script src="/static/theme.js?v=__THEME__"></script><link rel="stylesheet" href="/static/app.css?v=__STYLESHEET__"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/">{{icon "back"}} Library</a><header class="settings-intro"><span class="eyebrow">Owner controls</span><h1>System</h1><p>Review local Server activity, recent playback, and safe diagnostics in one place.</p><p><a class="mode" href="/settings">Server settings</a></p></header><nav class="settings-nav" aria-label="System sections"><a href="#logs">Recent activity</a><a href="#playback">Recent playback</a><a href="#diagnostics">Diagnostics</a></nav><div class="settings-flow"><section class="wide" id="logs"><h2>Recent activity</h2><p>The local audit journal records security and administration events without request bodies, secrets, or media paths.</p><p><a class="mode" href="/api/v1/activity/export">Export full activity journal</a></p><div class="activity-list">{{range .Logs}}<details class="activity-row"><summary><strong>{{.Action}}</strong><span class="activity-result">{{.Result}}{{if .Actor}} · {{.Actor}}{{end}}</span><time>{{.Time}}</time></summary>{{if or .Target .RequestID .Details}}<p>{{if .Target}}{{.Target}}{{end}}{{if .RequestID}} · request <code>{{.RequestID}}</code>{{end}}{{range $key, $value := .Details}} · {{$key}}=<code>{{$value}}</code>{{end}}</p>{{end}}</details>{{else}}<p>No activity recorded yet.</p>{{end}}</div></section><section class="wide" id="playback"><h2>Recent playback</h2><p>Playback history stays on this Server and is visible only to Owners.</p>{{range .Playback}}<p><time>{{.Updated}}</time> · <strong>{{.Profile}}</strong> · {{.Title}} · {{.Position}}</p>{{else}}<p>No playback recorded yet.</p>{{end}}</section><section id="diagnostics"><h2>Diagnostics</h2><p><strong class="status">{{if .Diagnostics.Healthy}}Healthy{{else}}Needs attention{{end}}</strong></p><p>Version: <code>{{.Diagnostics.Version}}</code></p><p>Library items: {{.Diagnostics.LibraryItems}} · Active sessions: {{.Diagnostics.Sessions}}</p><p>Playback: {{t .Diagnostics.PlaybackMode}} · Transcoder: {{t .Diagnostics.Transcoder}}</p><p>Library monitoring: {{.Diagnostics.LibraryMonitoring}} · Last scan: {{.Diagnostics.LastScan}}</p><p>HTTP requests: {{.Diagnostics.HTTPRequests}} · errors: {{.Diagnostics.HTTPErrors}} · panics: {{.Diagnostics.HTTPPanics}}</p><p>Activity journal: {{if .Diagnostics.ActivityHealthy}}healthy{{else}}needs attention{{end}} · Failed writes: {{.Diagnostics.ActivityFailures}}</p><p><a class="mode" href="/settings/diagnostics.json">Download safe diagnostics</a> <a class="mode" href="/settings/metrics">Metrics</a></p></section></div></main></body></html>`

// SystemViewSource derives one branded view from Player's canonical markup.
func SystemViewSource(brand SystemBrand) string {
	if brand.Product != "Player" && brand.Product != "Subtitles" || !systemAssetVersion(brand.IconVersion) || !systemAssetVersion(brand.ThemeVersion) || !systemAssetVersion(brand.StylesheetVersion) {
		panic("invalid system view configuration")
	}
	return strings.NewReplacer(
		"__PRODUCT__", brand.Product,
		"__ICON__", brand.IconVersion,
		"__THEME__", brand.ThemeVersion,
		"__STYLESHEET__", brand.StylesheetVersion,
	).Replace(systemViewSource)
}

func systemAssetVersion(value string) bool {
	if value == "" || len(value) > 8 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// SystemSource supplies the app-owned snapshots used by the System page.
type SystemSource[Playback any] struct {
	Logs        func() []auditjournal.Event
	Playback    func() []Playback
	Diagnostics func() DiagnosticReport
}

// SystemPage is the complete safe System page projection.
type SystemPage[Playback any] struct {
	Logs        []auditjournal.Event
	Playback    []Playback
	Diagnostics DiagnosticReport
}

type systemView interface {
	Execute(http.ResponseWriter, *http.Request, any) error
}

// NewSystemHandler binds app-owned snapshots to the shared System page.
func NewSystemHandler[Playback any](source SystemSource[Playback], view systemView) http.HandlerFunc {
	if source.Logs == nil || source.Playback == nil || source.Diagnostics == nil || view == nil {
		panic("invalid system page dependencies")
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		data := SystemPage[Playback]{source.Logs(), source.Playback(), source.Diagnostics()}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := view.Execute(writer, request, data); err != nil {
			slog.Error("render system", "error", err)
		}
	}
}
