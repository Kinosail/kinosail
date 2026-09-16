package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/library"
)

const downloadsHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta name="htmx-config" content='{"includeIndicatorStyles":false}'><title>Offline downloads · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/htmx.min.js"></script><script defer src="/static/downloads.js?v=3"></script></head><body class="detail-page"><main id="downloads" class="detail-shell downloads-shell" data-viewer-profile="{{.Profile}}" {{if .Pending}}hx-get="/offline-downloads" hx-trigger="every 3s" hx-select="#downloads" hx-target="#downloads" hx-swap="outerHTML"{{end}}><a class="back" href="/">{{icon "back"}} Library</a><span class="eyebrow">Downloads</span><h1>Offline downloads</h1><p>Prepared files are private to this Viewer Profile. Download a prepared file to this device to use it offline.</p>{{range .Jobs}}<article class="download-job" data-download-job="{{.ID}}"><header><div><h2>{{.Title}}</h2><p>{{.Quality}}</p></div><strong class="status">{{if .ReadyOffline}}Ready to download{{else if .Error}}Needs attention{{else}}Preparing{{end}}</strong></header>{{if .ReadyOffline}}<p role="status">{{.Quality}} · Ready to download</p><p>File verified on the Server · {{.Size}} bytes</p><div class="download-device-actions"><button type="button" class="mode" data-download-device data-job-id="{{.ID}}" data-item-id="{{.ItemID}}" data-title="{{.Title}}" data-quality="{{.Quality}}">Download to this device</button><span role="status" aria-live="polite" data-download-device-status>Not stored on this device</span></div><a class="mode" href="/api/v1/downloads/{{.ID}}/file">Save file</a>{{else if .Error}}<div role="alert"><p>{{.Error}}</p><form action="/offline/{{.ItemID}}" method="post"><button name="quality" value="{{.Quality}}">Try again</button></form></div>{{else}}<p role="status" aria-live="polite">Preparing {{.Title}} for offline use…</p><progress aria-label="Preparing {{.Title}}"></progress>{{end}}<details><summary>Remove download</summary><form action="/offline-downloads/{{.ID}}/remove" method="post"><button class="danger">Remove download</button></form></details></article>{{else}}<div class="empty"><h2>No offline downloads yet.</h2><p>Open a movie, episode, song, or audiobook and prepare an offline copy.</p></div>{{end}}</main></body></html>`

var downloadsView = newLocalizedTemplate("downloads", downloadsTemplate(downloadsHTML))

func downloadsTemplate(template string) string {
	template = strings.Replace(template, `/static/downloads.js?v=3`, `/static/downloads.js?v=8`, 1)
	return strings.Replace(template, `{{if .Pending}}hx-get="/offline-downloads" hx-trigger="every 3s" hx-select="#downloads" hx-target="#downloads" hx-swap="outerHTML"{{end}}`, `data-downloads-pending="{{.Pending}}"`, 1)
}

func (manager *downloadManager) registerWeb(mux *http.ServeMux, index *libraryIndex) {
	downloads.RegisterWeb(mux, manager.Manager, downloadAccess(index), downloadsView, localizedError, localizedNotFound)
}

func (manager *downloadManager) registerAPI(mux *http.ServeMux, index *libraryIndex) { //nolint:cyclop,gocognit // Registration keeps the download API lifecycle together.
	downloads.RegisterAPI(mux, manager.Manager, downloadAccess(index))
}

func downloadAccess(index *libraryIndex) downloads.Access {
	return downloads.NewAccess(currentViewer, func(request *http.Request, id string) (library.Item, bool) {
		return visibleItem(request, index, id)
	})
}
